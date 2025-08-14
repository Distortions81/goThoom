package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/google/gopacket/tcpassembly"
)

func replayPCAP(ctx context.Context, path string) error {
	// Ebiten must be running before ReadPixels is invoked, so wait for the game
	// to start before opening the PCAP. Propagate context cancellation so that
	// shutdown does not deadlock while waiting.
	select {
	case <-gameStarted:
	case <-ctx.Done():
		return ctx.Err()
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var (
		source *gopacket.PacketSource
	)
	if ng, err := pcapgo.NewNgReader(f, pcapgo.NgReaderOptions{}); err == nil {
		source = gopacket.NewPacketSource(ng, ng.LinkType())
	} else {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		r, err := pcapgo.NewReader(f)
		if err != nil {
			return err
		}
		source = gopacket.NewPacketSource(r, r.LinkType())
	}

	factory := &pcapStreamFactory{}
	pool := tcpassembly.NewStreamPool(factory)
	assembler := tcpassembly.NewAssembler(pool)

	for {
		select {
		case <-ctx.Done():
			assembler.FlushAll()
			return ctx.Err()
		default:
		}
		pkt, err := source.NextPacket()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		net := pkt.NetworkLayer()
		if net == nil {
			continue
		}
		transport := pkt.TransportLayer()
		if transport == nil {
			continue
		}
		switch t := transport.(type) {
		case *layers.UDP:
			handlePayload(t.Payload)
		case *layers.TCP:
			assembler.AssembleWithTimestamp(net.NetworkFlow(), t, pkt.Metadata().CaptureInfo.Timestamp)
		}
	}
	assembler.FlushAll()
	return nil
}

func handlePayload(p []byte) {
	if len(p) < 2 {
		return
	}
	sz := int(binary.BigEndian.Uint16(p[:2]))
	if len(p) < 2+sz {
		return
	}
	msg := p[2 : 2+sz]
	dispatchMessage(msg)
}

type pcapStreamFactory struct{}

func (f *pcapStreamFactory) New(net, transport gopacket.Flow) tcpassembly.Stream {
	return &pcapStream{}
}

type pcapStream struct {
	buf bytes.Buffer
}

func (s *pcapStream) Reassembled(rs []tcpassembly.Reassembly) {
	for _, r := range rs {
		if len(r.Bytes) > 0 {
			s.buf.Write(r.Bytes)
		}
	}
	for {
		b := s.buf.Bytes()
		if len(b) < 2 {
			return
		}
		l := int(binary.BigEndian.Uint16(b[:2]))
		if len(b) < 2+l {
			return
		}
		msg := append([]byte(nil), b[2:2+l]...)
		dispatchMessage(msg)
		s.buf.Next(2 + l)
	}
}

func (s *pcapStream) ReassemblyComplete() {}

func dispatchMessage(msg []byte) {
	if len(msg) < 2 {
		return
	}
	tag := binary.BigEndian.Uint16(msg[:2])
	if tag == 2 {
		handleDrawState(msg)
		return
	}
	if txt := decodeMessage(msg); txt != "" {
		consoleMessage("pcap: " + txt)
	}
}
