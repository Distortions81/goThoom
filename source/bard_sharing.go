package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gothoom/eui"
)

const (
	// Private thoughts can be truncated well below the player-command limit.
	// Keep outgoing bodies conservative, including the header, with room for
	// sender captions. Continue accepting intact segments from older clients.
	bardShareSendMessageBytes = 180
	bardShareMaxMessage       = 400
	bardShareMaxChunks        = 64
	bardShareMaxPayload       = 8192
	bardShareMaxReceivedParts = 16
	bardShareExpiry           = 2 * time.Minute
	bardShareBase58           = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
)

type bardSharedPart struct {
	Title      string
	Instrument int
	Notes      string
}

type bardIncomingPart struct {
	id      string
	part    bardSharedPart
	chunks  []string
	expires time.Time
}

type bardSharing struct {
	win                               *eui.WindowData
	analysis                          *eui.ItemData
	receive, send, cancelSend, status *eui.ItemData
	assignments                       []*eui.ItemData
	assignmentPath, assignmentValue   string
	session                           *Session
	generation                        uint64
	enabledAt                         time.Time
	partners                          string
	incoming                          map[string]*bardIncomingPart
	received                          map[string]time.Time
	receivedCount                     int
	tickets                           []CommandTicket
	sendSession                       *Session
	sendGeneration                    uint64
}

func bardPlayerKey(name string) string {
	options, err := bardWithOptions([]string{name})
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(options, "/with ")))
}

func bardSharingPartners(value, self string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("Choose one partner for a duet or two for a trio.")
	}
	names := strings.Split(value, ",")
	if _, err := bardWithOptions(names); err != nil {
		return nil, err
	}
	for i, name := range names {
		names[i] = strings.TrimSpace(name)
		if bardPlayerKey(name) == bardPlayerKey(self) {
			return nil, fmt.Errorf("Enter the other performers, without your own name.")
		}
	}
	return names, nil
}

func validateBardSharedPart(part bardSharedPart) error {
	if strings.TrimSpace(part.Title) == "" || len(part.Title) > 240 || !utf8.ValidString(part.Title) {
		return fmt.Errorf("Song names must be nonempty UTF-8 text up to 240 bytes.")
	}
	for _, r := range part.Title {
		if unicode.IsControl(r) {
			return fmt.Errorf("Song names cannot contain line breaks or control characters.")
		}
	}
	if part.Instrument < 0 || part.Instrument >= len(instruments) {
		return fmt.Errorf("Unknown instrument in shared part.")
	}
	// Validate plain notation, not metadata or commands embedded in the notes.
	if _, err := bardNoteTokens(part.Notes); err != nil {
		return err
	}
	if _, err := validateBardTune(part.Notes, part.Instrument); err != nil {
		return err
	}
	_, err := bardEnsembleCommands(part.Notes, []string{strings.Repeat("A", 32), strings.Repeat("B", 32)})
	return err
}

// Only delimiters and literal backslashes need escaping in the comment. Normal
// names stay readable, including MacRoman accents. These escapes stay inside
// CL comments, so a player can paste the complete messages into any tune editor.
func bardShareLabel(label string) string {
	escaped := strings.NewReplacer("%", "%25", "|", "%7C", "<", "%3C", ">", "%3E", "\\", "%5C").Replace(label)
	// Escape non-MacRoman characters before the general chat encoder can turn
	// emoji into shortcodes. goThoom restores these when reading server text.
	return decodeMacRoman(encodeMacRoman(escaped))
}

// CRC-16/CCITT-FALSE: polynomial 0x1021, initial value 0xffff, no reflection
// or final XOR. Encode the result as three Base58 digits, padding with '1'.
func bardSharedPartChecksum(part bardSharedPart) string {
	return bardShareChecksum(fmt.Sprintf("%s\x00%d\x00%s", part.Title, part.Instrument, part.Notes))
}

func bardShareChecksum(value string) string {
	crc := uint16(0xffff)
	for i := 0; i < len(value); i++ {
		crc ^= uint16(value[i]) << 8
		for bit := 0; bit < 8; bit++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	var encoded [3]byte
	for i := len(encoded) - 1; i >= 0; i-- {
		encoded[i] = bardShareBase58[crc%58]
		crc /= 58
	}
	return string(encoded[:])
}

func bardPartMessage(part bardSharedPart, id string, index, count int, notes string) string {
	return fmt.Sprintf("<%d of %d | %s | %s | %s> %s",
		index+1, count, bardShareLabel(part.Title), classicInstrumentNames[part.Instrument], id, notes)
}

func bardPartMessages(title string, part bardPart) ([]string, error) {
	tokens, err := bardNoteTokens(part.Text)
	if err != nil {
		return nil, err
	}
	// Whitespace between tokens has no musical meaning. Canonical spacing lets
	// recipients copy segments with line breaks or spaces without changing notes.
	var notes, canonical []string
	for _, token := range tokens {
		if strings.TrimSpace(token) != "" {
			canonical = append(canonical, token)
			notes = append(notes, token)
		} else if len(notes) > 0 && notes[len(notes)-1] != " " {
			notes = append(notes, " ")
		}
	}
	payload := bardSharedPart{Title: title, Instrument: part.Instrument, Notes: strings.Join(canonical, "")}
	if err := validateBardSharedPart(payload); err != nil {
		return nil, err
	}
	if len(payload.Title)+len(payload.Notes) > bardShareMaxPayload {
		return nil, fmt.Errorf("This part is too large to share.")
	}
	id := bardSharedPartChecksum(payload)
	// Reserve the widest segment numbers so determining the total never causes
	// a message to grow beyond the wire limit. Musical tokens are never split.
	header := bardPartMessage(payload, id, bardShareMaxChunks-1, bardShareMaxChunks, "")
	limit := bardShareSendMessageBytes - len(encodeMacRoman(encodeEmojiShortcodes(header)))
	if limit < 1 {
		return nil, fmt.Errorf("The song name is too long for a music message. Shorten it before sharing.")
	}
	var chunks []string
	chunk := ""
	for _, token := range notes {
		if len(token) > limit {
			return nil, fmt.Errorf("A music token cannot fit beside the song details. Shorten the song name.")
		}
		if len(chunk)+len(token) > limit {
			if strings.TrimSpace(chunk) != "" {
				chunks = append(chunks, strings.TrimSpace(chunk))
			}
			chunk = ""
		}
		chunk += token
	}
	if strings.TrimSpace(chunk) != "" {
		chunks = append(chunks, strings.TrimSpace(chunk))
	}
	if len(chunks) > bardShareMaxChunks {
		return nil, fmt.Errorf("This part needs too many messages. Shorten it before sharing.")
	}
	messages := make([]string, len(chunks))
	for n, chunk := range chunks {
		messages[n] = bardPartMessage(payload, id, n, len(chunks), chunk)
	}
	return messages, nil
}

func parseBardPartMessage(message string) (part bardSharedPart, id string, index, count int, chunk string, err error) {
	fail := fmt.Errorf("Invalid music part message.")
	if len(message) > bardShareMaxMessage*4 || len(encodeMacRoman(message)) > bardShareMaxMessage || !strings.HasPrefix(message, "<") {
		err = fail
		return
	}
	header, notes, ok := strings.Cut(message, "> ")
	if !ok {
		err = fail
		return
	}
	fields := strings.Split(strings.TrimPrefix(header, "<"), " | ")
	if len(fields) != 4 {
		err = fail
		return
	}
	numbering := strings.Split(fields[0], " of ")
	if len(numbering) != 2 {
		err = fail
		return
	}
	var e1, e2 error
	index, e1 = strconv.Atoi(numbering[0])
	count, e2 = strconv.Atoi(numbering[1])
	if e1 != nil || e2 != nil || index < 1 || index > count || count > bardShareMaxChunks {
		err = fail
		return
	}
	values := make([]string, 3)
	for n, value := range fields[1:] {
		if strings.ContainsAny(value, "<>|") {
			err = fail
			return
		}
		value = decodeServerText(encodeMacRoman(value))
		values[n], err = url.PathUnescape(value)
		if err != nil {
			err = fail
			return
		}
	}
	part = bardSharedPart{Title: values[0], Instrument: bardInstrumentIndex(values[1])}
	id = values[2]
	if part.Instrument < 0 || len(id) != 3 {
		err = fail
		return
	}
	for _, r := range id {
		if !strings.ContainsRune(bardShareBase58, r) {
			err = fail
			return
		}
	}
	if strings.TrimSpace(notes) == "" || strings.ContainsAny(notes, "<>;\r\n") {
		err = fail
		return
	}
	index--
	chunk = notes
	return
}

func decodeBardSharedPart(incoming *bardIncomingPart) (bardSharedPart, error) {
	part := incoming.part
	raw := strings.Join(incoming.chunks, "")
	if len(part.Title)+len(raw) > bardShareMaxPayload || !utf8.ValidString(raw) {
		return part, fmt.Errorf("The received part is too large or is not UTF-8 text.")
	}
	tokens, err := bardNoteTokens(raw)
	if err != nil {
		return part, err
	}
	for _, token := range tokens {
		if strings.TrimSpace(token) != "" {
			part.Notes += token
		}
	}
	if bardSharedPartChecksum(part) != incoming.id {
		return part, fmt.Errorf("The received part is incomplete or damaged. Ask the sender to resend it.")
	}
	return part, validateBardSharedPart(part)
}

// Called only for private thoughts decoded from server bubble records, never
// from formatted chat, emotes, or claimed senders inside the payload.
func queueBardPartMessage(session *Session, sender, message string) {
	if !strings.HasPrefix(message, "<") || !strings.Contains(message, " of ") || !strings.Contains(message, " | ") || len(message) > bardShareMaxMessage*4 {
		return
	}
	generation, receivedAt := bardConnectionGeneration(session), time.Now()
	dispatchMainThread(func() {
		if p := bardPanels[session]; p != nil {
			p.receivePartMessage(session, generation, sender, message, receivedAt)
		}
	})
}

func (p *bardPanel) setReceiveParts(enabled bool) {
	share := &p.sharing
	share.receive.Checked = enabled
	share.incoming = nil
	share.received = nil
	share.receivedCount = 0
	share.session = p.session
	share.enabledAt = time.Now()
	share.partners = p.partners.Text
	if share.session != nil {
		share.generation = bardConnectionGeneration(share.session)
	}
}

func (p *bardPanel) receivePartMessage(session *Session, generation uint64, sender, message string, at time.Time) {
	share := &p.sharing
	if session == nil || playingMovie || session == moviePlaybackSession || share.receive == nil || !share.receive.Checked || !p.registered() || session != share.session || generation != share.generation || !session.transport.connectedGeneration(generation) || at.Before(share.enabledAt) || time.Since(at) > bardShareExpiry {
		return
	}
	if share.partners != p.partners.Text {
		share.incoming = nil
		share.partners = p.partners.Text
	}
	names, err := bardSharingPartners(p.partners.Text, session.characterName())
	if err != nil {
		return
	}
	key := bardPlayerKey(sender)
	allowed := false
	for _, name := range names {
		allowed = allowed || key != "" && key == bardPlayerKey(name)
	}
	if !allowed {
		return
	}
	if player, ok := playerSnapshotForSession(session, sender); ok && (player.Blocked || player.Ignored) {
		return
	}
	metadata, id, index, count, chunk, err := parseBardPartMessage(message)
	if err != nil {
		return
	}
	now := time.Now()
	if share.incoming == nil {
		share.incoming = make(map[string]*bardIncomingPart)
	}
	if share.received == nil {
		share.received = make(map[string]time.Time)
	}
	for k, expiry := range share.received {
		if now.After(expiry) {
			delete(share.received, k)
		}
	}
	if _, done := share.received[key+":"+id]; done {
		return
	}
	incoming := share.incoming[key]
	if incoming == nil || incoming.id != id || now.After(incoming.expires) {
		incoming = &bardIncomingPart{id: id, part: metadata, chunks: make([]string, count), expires: now.Add(bardShareExpiry)}
		share.incoming[key] = incoming
	}
	if incoming.part != metadata || len(incoming.chunks) != count || incoming.chunks[index] != "" && incoming.chunks[index] != chunk {
		delete(share.incoming, key)
		return
	}
	// Bound incomplete transfers too, before retaining any additional data.
	size := len(metadata.Title) + len(chunk)
	for i, existing := range incoming.chunks {
		if i != index {
			size += len(existing)
		}
	}
	if size > bardShareMaxPayload {
		delete(share.incoming, key)
		p.setError(fmt.Errorf("Part from %s was rejected: Music parts are limited to 8 KiB.", sender))
		return
	}
	incoming.chunks[index] = chunk
	for _, chunk := range incoming.chunks {
		if chunk == "" {
			return
		}
	}
	delete(share.incoming, key)
	// Bound disk writes across the entire receiving period, even after old
	// duplicate records expire. Only explicitly enabling receiving resets it.
	if share.receivedCount >= bardShareMaxReceivedParts {
		p.setReceiveParts(false)
		p.setError(fmt.Errorf("Part receiving paused after %d saved parts. Enable it again in Duet / Trio when ready.", bardShareMaxReceivedParts))
		return
	}
	part, err := decodeBardSharedPart(incoming)
	if err != nil {
		p.setError(fmt.Errorf("Part from %s was rejected: %w", sender, err))
		return
	}
	text := strings.Join([]string{
		bardMetadata("title", part.Title),
		bardComment("Received from " + sender),
		bardMetadata("instrument", classicInstrumentNames[part.Instrument]),
		part.Notes, "",
	}, "\n")
	// Only locally generated names become paths. Never overwrite an existing tune.
	name := fmt.Sprintf("Received %s %s", time.Now().Format("20060102-150405.000000000"), id)
	tune, err := createBardTune(name, text)
	if err != nil {
		p.setError(err)
		return
	}
	share.received[key+":"+id] = now.Add(bardShareExpiry)
	share.receivedCount++
	if p.playConfirm != nil {
		p.playConfirm.Close()
	}
	p.selected, p.selectedPart = tune.Path, "Solo"
	p.query = ""
	p.tag.Selected = 0
	p.reload()
	p.setStatus(fmt.Sprintf("Received %s from %s. Ready for Play in Game.", part.Title, sender), false)
}
