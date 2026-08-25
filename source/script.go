package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	scriptapi "gt2"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

const scriptAPICurrentVersion = 2

type scriptScope struct {
	All   bool
	Chars map[string]bool
}

func (s scriptScope) enablesFor(effChar string) bool {
	if s.All {
		return true
	}
	if effChar == "" || s.Chars == nil {
		return false
	}
	return s.Chars[effChar]
}

func (s *scriptScope) addChar(name string) {
	if name == "" {
		return
	}
	if s.Chars == nil {
		s.Chars = map[string]bool{}
	}
	s.Chars[name] = true
}

func (s *scriptScope) removeChar(name string) {
	if s.Chars == nil || name == "" {
		return
	}
	delete(s.Chars, name)
}

func (s scriptScope) empty() bool { return !s.All && len(s.Chars) == 0 }

// Expose the script API at its single documented import path.
var basescriptExports = interp.Exports{
	// Short path used by simple script scripts: import "gt2"
	// Yaegi expects keys as "importPath/pkgName".
	"gt2/gt2": {
		"ShowNotification":    reflect.ValueOf(scriptShowNotification),
		"CLVersion":           reflect.ValueOf(&clVersion).Elem(),
		"Self":                reflect.ValueOf(scriptSelf),
		"Players":             reflect.ValueOf(scriptPlayers),
		"Inventory":           reflect.ValueOf(scriptInventory),
		"Item":                reflect.ValueOf((*scriptapi.Item)(nil)),
		"SlotForehead":        reflect.ValueOf(scriptapi.SlotForehead),
		"SlotNeck":            reflect.ValueOf(scriptapi.SlotNeck),
		"SlotShoulder":        reflect.ValueOf(scriptapi.SlotShoulder),
		"SlotArms":            reflect.ValueOf(scriptapi.SlotArms),
		"SlotGloves":          reflect.ValueOf(scriptapi.SlotGloves),
		"SlotFinger":          reflect.ValueOf(scriptapi.SlotFinger),
		"SlotCoat":            reflect.ValueOf(scriptapi.SlotCoat),
		"SlotCloak":           reflect.ValueOf(scriptapi.SlotCloak),
		"SlotTorso":           reflect.ValueOf(scriptapi.SlotTorso),
		"SlotWaist":           reflect.ValueOf(scriptapi.SlotWaist),
		"SlotLegs":            reflect.ValueOf(scriptapi.SlotLegs),
		"SlotFeet":            reflect.ValueOf(scriptapi.SlotFeet),
		"SlotRightHand":       reflect.ValueOf(scriptapi.SlotRightHand),
		"SlotLeftHand":        reflect.ValueOf(scriptapi.SlotLeftHand),
		"SlotBothHands":       reflect.ValueOf(scriptapi.SlotBothHands),
		"SlotHead":            reflect.ValueOf(scriptapi.SlotHead),
		"Player":              reflect.ValueOf((*scriptapi.Player)(nil)),
		"Character":           reflect.ValueOf((*scriptapi.Character)(nil)),
		"PlaySound":           reflect.ValueOf(scriptPlaySound),
		"InputText":           reflect.ValueOf(scriptInputText),
		"SetInputText":        reflect.ValueOf(scriptSetInputText),
		"LastClick":           reflect.ValueOf(scriptLastClick),
		"Hover":               reflect.ValueOf(scriptHover),
		"SelectedPlayer":      reflect.ValueOf(scriptSelectedPlayer),
		"SelectedItem":        reflect.ValueOf(scriptSelectedItem),
		"CurrentWorld":        reflect.ValueOf(scriptCurrentWorld),
		"Click":               reflect.ValueOf((*scriptapi.Click)(nil)),
		"World":               reflect.ValueOf((*scriptapi.World)(nil)),
		"InputEvent":          reflect.ValueOf((*InputEvent)(nil)),
		"ChatFilter":          reflect.ValueOf((*ChatFilter)(nil)),
		"ChatEvent":           reflect.ValueOf((*ChatEvent)(nil)),
		"ServerMessageFilter": reflect.ValueOf((*ServerMessageFilter)(nil)),
		"ServerMessage":       reflect.ValueOf((*scriptapi.ServerMessage)(nil)),
		"LatestServerMessage": reflect.ValueOf(scriptLatestServerMessage),
		"LifecycleEvent":      reflect.ValueOf((*LifecycleEvent)(nil)),
		"ChangeEvent":         reflect.ValueOf((*ChangeEvent)(nil)),
		"Subscription":        reflect.ValueOf((*Subscription)(nil)),
		"Timer":               reflect.ValueOf((*Timer)(nil)),
		"BoolOption":          reflect.ValueOf((*scriptapi.BoolOption)(nil)),
		"IntegerOption":       reflect.ValueOf((*scriptapi.IntegerOption)(nil)),
		"DecimalOption":       reflect.ValueOf((*scriptapi.DecimalOption)(nil)),
		"TextOption":          reflect.ValueOf((*scriptapi.TextOption)(nil)),
		"ChoiceOption":        reflect.ValueOf((*scriptapi.ChoiceOption)(nil)),
		"KeyBindingOption":    reflect.ValueOf((*scriptapi.KeyBindingOption)(nil)),
		"ItemOption":          reflect.ValueOf((*scriptapi.ItemOption)(nil)),
		"ToolbarButton":       reflect.ValueOf((*scriptapi.ToolbarButton)(nil)),
		"ToolbarOptions":      reflect.ValueOf((*scriptapi.ToolbarOptions)(nil)),
		"ScopeGlobal":         reflect.ValueOf(scriptapi.ScopeGlobal),
		"ScopeCharacter":      reflect.ValueOf(scriptapi.ScopeCharacter),
		"Mobile":              reflect.ValueOf((*Mobile)(nil)),
		"EquippedItems":       reflect.ValueOf(scriptEquippedItems),
		"FindItemExact":       reflect.ValueOf(scriptFindItemExact),
		"FindItem":            reflect.ValueOf(scriptFindItem),
		"FindItems":           reflect.ValueOf(scriptFindItems),
		"SearchItems":         reflect.ValueOf(scriptSearchItems),
		"Equipped":            reflect.ValueOf(scriptEquipped),
		"HasItem":             reflect.ValueOf(scriptHasItem),
		"IsEquipped":          reflect.ValueOf(scriptIsEquipped),
		// Chat trigger flags
		"ChatAny":              reflect.ValueOf(ChatAny),
		"ChatPlayer":           reflect.ValueOf(ChatPlayer),
		"ChatNPC":              reflect.ValueOf(ChatNPC),
		"ChatCreature":         reflect.ValueOf(ChatCreature),
		"ChatSelf":             reflect.ValueOf(ChatSelf),
		"ChatOther":            reflect.ValueOf(ChatOther),
		"ChatTypeSay":          reflect.ValueOf(ChatTypeSay),
		"ChatTypeYell":         reflect.ValueOf(ChatTypeYell),
		"ChatTypeWhisper":      reflect.ValueOf(ChatTypeWhisper),
		"ChatTypeAsk":          reflect.ValueOf(ChatTypeAsk),
		"ChatTypeExclaim":      reflect.ValueOf(ChatTypeExclaim),
		"ChatTypeThinkTo":      reflect.ValueOf(ChatTypeThinkTo),
		"ChatTypeEmote":        reflect.ValueOf(ChatTypeEmote),
		"ChangeInventory":      reflect.ValueOf(ChangeInventory),
		"ChangeEquipment":      reflect.ValueOf(ChangeEquipment),
		"ChangeSelectedPlayer": reflect.ValueOf(ChangeSelectedPlayer),
		"ChangeSelectedItem":   reflect.ValueOf(ChangeSelectedItem),
		"ChangeVitals":         reflect.ValueOf(ChangeVitals),
		"ChangeWorld":          reflect.ValueOf(ChangeWorld),
		"ChangeLocation":       reflect.ValueOf(ChangeLocation),
	},
}

type scriptCandidate struct {
	mu          sync.Mutex
	active      bool
	failed      bool
	actions     []func()
	storage     map[string]scriptCandidateStorage
	commands    map[string]struct{}
	bindings    []string
	conflicts   []string
	eventQueue  *scriptEventQueue
	assets      *scriptAssetSource
	terminating bool
}

type scriptCandidateStorage struct {
	value   any
	deleted bool
}

type scriptDispatchEntry struct {
	owner  string
	action func()
}

type scriptEvent struct {
	name     string
	callback func()
	done     chan bool
}

type scriptEventQueue struct {
	owner             string
	interpreter       *interp.Interpreter
	diagnostics       *scriptDiagnostics
	mu                sync.Mutex
	events            []scriptEvent
	running           bool
	stopped           bool
	activeLimiter     *scriptExecutionLimiter
	budgetWindowStart time.Time
	budgetUsed        time.Duration
	nextRegistration  uint64
	registrations     map[uint64]func()
	done              chan struct{}
}

type scriptRegistrationHandle struct {
	queue *scriptEventQueue
	id    uint64
}

type scriptSubscriptionState struct {
	mu      sync.Mutex
	handle  scriptRegistrationHandle
	removed bool
}

// Subscription is an opaque removable script registration.
type Subscription struct {
	state *scriptSubscriptionState
}

type scriptTimerState struct {
	mu      sync.Mutex
	stop    func()
	stopped bool
}

// Timer is an opaque independently stoppable repeating script timer.
type Timer struct {
	state *scriptTimerState
}

func newScriptTimer() Timer {
	return Timer{state: &scriptTimerState{}}
}

func (t Timer) attach(stop func()) {
	if stop == nil {
		return
	}
	if t.state == nil {
		stop()
		return
	}
	t.state.mu.Lock()
	if t.state.stopped {
		t.state.mu.Unlock()
		stop()
		return
	}
	t.state.stop = stop
	t.state.mu.Unlock()
}

func (t Timer) Stop() {
	if t.state == nil {
		return
	}
	t.state.mu.Lock()
	if t.state.stopped {
		t.state.mu.Unlock()
		return
	}
	t.state.stopped = true
	stop := t.state.stop
	t.state.stop = nil
	t.state.mu.Unlock()
	if stop != nil {
		stop()
	}
}

func (t Timer) Active() bool {
	if t.state == nil {
		return false
	}
	t.state.mu.Lock()
	defer t.state.mu.Unlock()
	return !t.state.stopped && t.state.stop != nil
}

func newScriptSubscription() Subscription {
	return Subscription{state: &scriptSubscriptionState{}}
}

func (s Subscription) attach(handle scriptRegistrationHandle) {
	if s.state == nil {
		handle.release()
		return
	}
	s.state.mu.Lock()
	if s.state.removed {
		s.state.mu.Unlock()
		handle.release()
		return
	}
	s.state.handle = handle
	s.state.mu.Unlock()
}

func (s Subscription) Remove() {
	if s.state == nil {
		return
	}
	s.state.mu.Lock()
	if s.state.removed {
		s.state.mu.Unlock()
		return
	}
	s.state.removed = true
	handle := s.state.handle
	s.state.handle = scriptRegistrationHandle{}
	s.state.mu.Unlock()
	handle.release()
}

func (s Subscription) Active() bool {
	if s.state == nil {
		return false
	}
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	return !s.state.removed && s.state.handle.valid()
}

func (h scriptRegistrationHandle) valid() bool {
	return h.queue != nil && h.id != 0
}

func (h scriptRegistrationHandle) release() {
	if !h.valid() {
		return
	}
	h.queue.mu.Lock()
	cleanup := h.queue.registrations[h.id]
	delete(h.queue.registrations, h.id)
	h.queue.mu.Unlock()
	if cleanup != nil {
		cleanup()
	}
}

func registerScriptResource(owner string, cleanup func()) scriptRegistrationHandle {
	queue := currentScriptEventQueue(owner)
	if queue == nil || cleanup == nil {
		return scriptRegistrationHandle{}
	}
	queue.mu.Lock()
	if queue.stopped {
		queue.mu.Unlock()
		return scriptRegistrationHandle{}
	}
	queue.nextRegistration++
	handle := scriptRegistrationHandle{queue: queue, id: queue.nextRegistration}
	if queue.registrations == nil {
		queue.registrations = map[uint64]func(){}
	}
	queue.registrations[handle.id] = cleanup
	queue.mu.Unlock()
	return handle
}

func releaseScriptRegistrations(queue *scriptEventQueue) {
	if queue == nil {
		return
	}
	queue.mu.Lock()
	registrations := queue.registrations
	queue.registrations = nil
	queue.mu.Unlock()
	ids := make([]uint64, 0, len(registrations))
	for id := range registrations {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for i := len(ids) - 1; i >= 0; i-- {
		if cleanup := registrations[ids[i]]; cleanup != nil {
			cleanup()
		}
	}
}

type scriptDiagnostics struct {
	mu      sync.Mutex
	pending string
	lines   []string
}

func (d *scriptDiagnostics) Write(data []byte) (int, error) {
	_, _ = os.Stderr.Write(data)
	d.mu.Lock()
	d.pending += string(data)
	for {
		newline := strings.IndexByte(d.pending, '\n')
		if newline < 0 {
			break
		}
		line := strings.TrimSpace(d.pending[:newline])
		d.pending = d.pending[newline+1:]
		if line != "" {
			d.lines = append(d.lines, line)
			if len(d.lines) > 20 {
				d.lines = d.lines[len(d.lines)-20:]
			}
		}
	}
	d.mu.Unlock()
	return len(data), nil
}

func (d *scriptDiagnostics) clear() {
	if d == nil {
		return
	}
	d.mu.Lock()
	d.pending = ""
	d.lines = nil
	d.mu.Unlock()
}

func (d *scriptDiagnostics) panicLocation() string {
	if d == nil {
		return ""
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := len(d.lines) - 1; i >= 0; i-- {
		line := d.lines[i]
		if at := strings.Index(line, ": panic:"); at >= 0 {
			location := strings.TrimPrefix(line[:at], "_.go:")
			return location
		}
	}
	return ""
}

type scriptExecutionLimiter struct {
	mu        sync.Mutex
	remaining time.Duration
	started   time.Time
	timer     *time.Timer
	timedOut  chan struct{}
	once      sync.Once
	paused    bool
}

var (
	scriptCallbackTimeLimit = 2 * time.Second
	scriptExecutionWindow   = 10 * time.Second
	scriptExecutionBudget   = 5 * time.Second
)

var (
	scriptDispatchMu    sync.Mutex
	scriptDispatchQueue []scriptDispatchEntry
	scriptEventMu       sync.Mutex
	scriptEventQueues   = map[string]*scriptEventQueue{}
)

func startScriptEventQueue(owner string, interpreters ...*interp.Interpreter) *scriptEventQueue {
	var interpreter *interp.Interpreter
	if len(interpreters) > 0 {
		interpreter = interpreters[0]
	}
	queue := &scriptEventQueue{owner: owner, interpreter: interpreter, done: make(chan struct{})}
	scriptEventMu.Lock()
	old := scriptEventQueues[owner]
	scriptEventQueues[owner] = queue
	scriptEventMu.Unlock()
	if old != nil {
		old.stop()
	}
	return queue
}

func currentScriptEventQueue(owner string) *scriptEventQueue {
	scriptEventMu.Lock()
	queue := scriptEventQueues[owner]
	scriptEventMu.Unlock()
	return queue
}

func scriptEventQueueIsCurrent(owner string, queue *scriptEventQueue) bool {
	if queue == nil {
		return false
	}
	scriptEventMu.Lock()
	current := scriptEventQueues[owner] == queue
	scriptEventMu.Unlock()
	return current
}

func stopScriptEventQueue(owner string) *scriptEventQueue {
	scriptEventMu.Lock()
	queue := scriptEventQueues[owner]
	delete(scriptEventQueues, owner)
	scriptEventMu.Unlock()
	if queue != nil {
		queue.stop()
	}
	return queue
}

func (q *scriptEventQueue) enqueue(event scriptEvent) bool {
	if q == nil || event.callback == nil {
		return false
	}
	q.mu.Lock()
	if q.stopped {
		q.mu.Unlock()
		return false
	}
	q.events = append(q.events, event)
	if q.running {
		q.mu.Unlock()
		return true
	}
	q.running = true
	q.mu.Unlock()
	go q.run()
	return true
}

func newScriptExecutionLimiter(limit time.Duration) *scriptExecutionLimiter {
	limiter := &scriptExecutionLimiter{
		remaining: limit,
		started:   time.Now(),
		timedOut:  make(chan struct{}),
	}
	limiter.scheduleLocked()
	return limiter
}

func (l *scriptExecutionLimiter) scheduleLocked() {
	if l.remaining <= 0 {
		l.once.Do(func() { close(l.timedOut) })
		return
	}
	l.started = time.Now()
	l.timer = time.AfterFunc(l.remaining, func() {
		l.once.Do(func() { close(l.timedOut) })
	})
}

func (l *scriptExecutionLimiter) pause() {
	if l == nil {
		return
	}
	l.mu.Lock()
	if l.paused {
		l.mu.Unlock()
		return
	}
	if l.timer != nil {
		l.timer.Stop()
	}
	l.remaining -= time.Since(l.started)
	l.paused = true
	l.mu.Unlock()
}

func (l *scriptExecutionLimiter) resume() {
	if l == nil {
		return
	}
	l.mu.Lock()
	if !l.paused {
		l.mu.Unlock()
		return
	}
	l.paused = false
	l.scheduleLocked()
	l.mu.Unlock()
}

func (l *scriptExecutionLimiter) finish(limit time.Duration) time.Duration {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	if l.timer != nil {
		l.timer.Stop()
	}
	remaining := l.remaining
	if !l.paused {
		remaining -= time.Since(l.started)
	}
	if remaining < 0 {
		remaining = 0
	}
	l.mu.Unlock()
	return limit - remaining
}

func (q *scriptEventQueue) callbackAllowance(now time.Time) (time.Duration, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.budgetWindowStart.IsZero() || now.Sub(q.budgetWindowStart) >= scriptExecutionWindow {
		q.budgetWindowStart = now
		q.budgetUsed = 0
	}
	remainingBudget := scriptExecutionBudget - q.budgetUsed
	if remainingBudget <= 0 {
		return 0, true
	}
	if remainingBudget < scriptCallbackTimeLimit {
		return remainingBudget, true
	}
	return scriptCallbackTimeLimit, false
}

func (q *scriptEventQueue) consumeBudget(duration time.Duration) {
	q.mu.Lock()
	q.budgetUsed += duration
	q.mu.Unlock()
}

func (q *scriptEventQueue) pauseExecution() {
	q.mu.Lock()
	limiter := q.activeLimiter
	q.mu.Unlock()
	limiter.pause()
}

func (q *scriptEventQueue) resumeExecution() {
	q.mu.Lock()
	limiter := q.activeLimiter
	q.mu.Unlock()
	limiter.resume()
}

func (q *scriptEventQueue) interruptInterpreter() {
	interruptScriptInterpreter(q.interpreter)
}

func interruptScriptInterpreter(interpreter *interp.Interpreter) {
	if interpreter == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = interpreter.EvalWithContext(ctx, `(func() { for {} })()`)
}

func (q *scriptEventQueue) execute(event scriptEvent) bool {
	allowance, budgetLimited := q.callbackAllowance(time.Now())
	if allowance <= 0 {
		if scriptExecutionLimitHandler != nil {
			scriptExecutionLimitHandler(q.owner, event.name, true)
		}
		return false
	}
	limiter := newScriptExecutionLimiter(allowance)
	q.diagnostics.clear()
	q.mu.Lock()
	q.activeLimiter = limiter
	q.mu.Unlock()
	result := make(chan bool, 1)
	go func() { result <- runScriptCallback(q.owner, event.name, event.callback) }()

	select {
	case ok := <-result:
		used := limiter.finish(allowance)
		q.mu.Lock()
		if q.activeLimiter == limiter {
			q.activeLimiter = nil
		}
		q.mu.Unlock()
		q.consumeBudget(used)
		return ok
	case <-limiter.timedOut:
		limiter.finish(allowance)
		q.mu.Lock()
		if q.activeLimiter == limiter {
			q.activeLimiter = nil
		}
		q.mu.Unlock()
		q.interruptInterpreter()
		if scriptExecutionLimitHandler != nil {
			scriptExecutionLimitHandler(q.owner, event.name, budgetLimited)
		}
		return false
	}
}

func (q *scriptEventQueue) run() {
	for {
		q.mu.Lock()
		if q.stopped || len(q.events) == 0 {
			q.running = false
			q.mu.Unlock()
			return
		}
		event := q.events[0]
		q.events[0] = scriptEvent{}
		q.events = q.events[1:]
		q.mu.Unlock()

		ok := q.execute(event)
		if event.done != nil {
			event.done <- ok
		}
		if !ok {
			q.stop()
			return
		}
	}
}

func (q *scriptEventQueue) stop() {
	if q == nil {
		return
	}
	q.mu.Lock()
	if q.stopped {
		q.mu.Unlock()
		return
	}
	q.stopped = true
	if q.done != nil {
		close(q.done)
	}
	pending := q.events
	q.events = nil
	q.mu.Unlock()
	for _, event := range pending {
		if event.done != nil {
			event.done <- false
		}
	}
}

func queueScriptCallback(owner, event string, callback func()) bool {
	if callback == nil || scriptIsDisabled(owner) {
		return false
	}
	queue := currentScriptEventQueue(owner)
	if queue == nil {
		return false
	}
	return queue.enqueue(scriptEvent{name: event, callback: callback})
}

func queueScriptCallbackWait(owner, event string, callback func()) bool {
	if callback == nil || scriptIsDisabled(owner) {
		return false
	}
	queue := currentScriptEventQueue(owner)
	if queue == nil {
		return false
	}
	done := make(chan bool, 1)
	if !queue.enqueue(scriptEvent{name: event, callback: callback, done: done}) {
		return false
	}
	return <-done
}

func queueScriptCallbackOn(queue *scriptEventQueue, owner, event string, callback func()) bool {
	if queue == nil {
		return runScriptCallback(owner, event, callback)
	}
	return queue.enqueue(scriptEvent{name: event, callback: callback})
}

func queueScriptCallbackWaitOn(queue *scriptEventQueue, owner, event string, callback func()) bool {
	if queue == nil {
		return runScriptCallback(owner, event, callback)
	}
	done := make(chan bool, 1)
	if !queue.enqueue(scriptEvent{name: event, callback: callback, done: done}) {
		return false
	}
	return <-done
}

func dispatchScript(owner string, action func()) {
	if action == nil || scriptIsDisabled(owner) {
		return
	}
	scriptDispatchMu.Lock()
	scriptDispatchQueue = append(scriptDispatchQueue, scriptDispatchEntry{owner: owner, action: action})
	scriptDispatchMu.Unlock()
}

func dispatchScriptControl(action func()) {
	if action == nil {
		return
	}
	scriptDispatchMu.Lock()
	scriptDispatchQueue = append(scriptDispatchQueue, scriptDispatchEntry{action: action})
	scriptDispatchMu.Unlock()
}

func cancelScriptDispatch(owner string) {
	scriptDispatchMu.Lock()
	kept := scriptDispatchQueue[:0]
	for _, entry := range scriptDispatchQueue {
		if entry.owner != owner {
			kept = append(kept, entry)
		}
	}
	scriptDispatchQueue = kept
	scriptDispatchMu.Unlock()
}

func drainScriptDispatcher() {
	scriptDispatchMu.Lock()
	queue := append([]scriptDispatchEntry(nil), scriptDispatchQueue...)
	scriptDispatchQueue = scriptDispatchQueue[:0]
	scriptDispatchMu.Unlock()
	for _, entry := range queue {
		if entry.action == nil {
			continue
		}
		if entry.owner == "" {
			entry.action()
			continue
		}
		runScriptCallback(entry.owner, "Dispatch", entry.action)
	}
}

func normalizeScriptCommand(name string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "/")))
}

func (c *scriptCandidate) claimCommand(name string, handler scriptCommandHandler) bool {
	if c == nil {
		return true
	}
	key := normalizeScriptCommand(name)
	if key == "" || handler == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failed {
		return false
	}
	if c.active {
		return true
	}
	if c.commands == nil {
		c.commands = map[string]struct{}{}
	}
	if _, exists := c.commands[key]; exists {
		c.conflicts = append(c.conflicts, "duplicate command /"+key+" in the same script")
		return false
	}
	c.commands[key] = struct{}{}
	return true
}

func (c *scriptCandidate) claimBinding(combo string, handler any) bool {
	if c == nil {
		return true
	}
	combo = strings.TrimSpace(combo)
	if combo == "" || handler == nil || (reflect.ValueOf(handler).Kind() == reflect.Func && reflect.ValueOf(handler).IsNil()) {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failed {
		return false
	}
	if c.active {
		return true
	}
	for _, existing := range c.bindings {
		if sameCombo(existing, combo) {
			c.conflicts = append(c.conflicts, "duplicate binding "+combo+" in the same script")
			return false
		}
	}
	c.bindings = append(c.bindings, combo)
	return true
}

func (c *scriptCandidate) dispatch(owner string, action func()) {
	if c == nil {
		dispatchScript(owner, action)
		return
	}
	c.mu.Lock()
	if c.failed {
		c.mu.Unlock()
		return
	}
	if !c.active {
		c.actions = append(c.actions, action)
		c.mu.Unlock()
		return
	}
	eventQueue := c.eventQueue
	c.mu.Unlock()
	if !scriptEventQueueIsCurrent(owner, eventQueue) {
		return
	}
	dispatchScript(owner, action)
}

func (c *scriptCandidate) runtimeEventQueue(owner string) *scriptEventQueue {
	if c == nil {
		return currentScriptEventQueue(owner)
	}
	c.mu.Lock()
	active := c.active && !c.failed
	queue := c.eventQueue
	c.mu.Unlock()
	if !active {
		return nil
	}
	return queue
}

func (c *scriptCandidate) setStorage(owner, key string, value any) {
	if c == nil {
		scriptStorageSet(owner, key, value)
		return
	}
	if err := validateScriptStorageValue(value); err != nil {
		reportScriptStorageError(owner, key, err)
		return
	}
	c.mu.Lock()
	if c.failed {
		c.mu.Unlock()
		return
	}
	if c.active {
		eventQueue := c.eventQueue
		terminating := c.terminating
		c.mu.Unlock()
		if !terminating && (scriptIsDisabled(owner) || !scriptEventQueueIsCurrent(owner, eventQueue)) {
			return
		}
		setScriptStorageValue(owner, key, value)
		return
	}
	if c.storage == nil {
		c.storage = map[string]scriptCandidateStorage{}
	}
	c.storage[key] = scriptCandidateStorage{value: value}
	c.actions = append(c.actions, func() { setScriptStorageValue(owner, key, value) })
	c.mu.Unlock()
}

func (c *scriptCandidate) getStorage(owner, key string) any {
	if c == nil {
		return scriptStorageGet(owner, key)
	}
	c.mu.Lock()
	if !c.active && !c.failed {
		if value, ok := c.storage[key]; ok {
			c.mu.Unlock()
			if value.deleted {
				return nil
			}
			return value.value
		}
	}
	c.mu.Unlock()
	return scriptStorageGet(owner, key)
}

func (c *scriptCandidate) deleteStorage(owner, key string) {
	if c == nil {
		scriptStorageDelete(owner, key)
		return
	}
	c.mu.Lock()
	if c.failed {
		c.mu.Unlock()
		return
	}
	if c.active {
		eventQueue := c.eventQueue
		terminating := c.terminating
		c.mu.Unlock()
		if !terminating && (scriptIsDisabled(owner) || !scriptEventQueueIsCurrent(owner, eventQueue)) {
			return
		}
		scriptStorageDelete(owner, key)
		return
	}
	if c.storage == nil {
		c.storage = map[string]scriptCandidateStorage{}
	}
	c.storage[key] = scriptCandidateStorage{deleted: true}
	c.actions = append(c.actions, func() { scriptStorageDelete(owner, key) })
	c.mu.Unlock()
}

func (c *scriptCandidate) activate(eventQueue *scriptEventQueue) {
	c.mu.Lock()
	if c.failed || c.active {
		c.mu.Unlock()
		return
	}
	c.active = true
	c.eventQueue = eventQueue
	actions := append([]func(){}, c.actions...)
	c.actions = nil
	c.storage = nil
	c.mu.Unlock()
	for _, action := range actions {
		action()
	}
}

func (c *scriptCandidate) discard() {
	c.mu.Lock()
	c.failed = true
	c.actions = nil
	c.storage = nil
	c.commands = nil
	c.bindings = nil
	c.eventQueue = nil
	c.terminating = false
	c.mu.Unlock()
}

func (c *scriptCandidate) callTerminate(fn func()) {
	c.mu.Lock()
	c.terminating = true
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.terminating = false
		c.mu.Unlock()
	}()
	fn()
}

func exportsForscript(owner string) interp.Exports {
	return exportsForScriptCandidate(owner, nil)
}

func exportsForScriptCandidate(owner string, candidate *scriptCandidate) interp.Exports {
	ex := make(interp.Exports)
	for pkg, symbols := range basescriptExports {
		m := map[string]reflect.Value{}
		for k, v := range symbols {
			m[k] = v
		}
		stage := func(action func()) { candidate.dispatch(owner, action) }
		subscribe := func(register func() scriptRegistrationHandle) Subscription {
			subscription := newScriptSubscription()
			stage(func() { subscription.attach(register()) })
			return subscription
		}
		m["Equip"] = reflect.ValueOf(func(name string) {
			if candidate.runtimeEventQueue(owner) != nil {
				scriptEquipByName(owner, name)
				return
			}
			stage(func() { scriptEquipByName(owner, name) })
		})
		m["Unequip"] = reflect.ValueOf(func(name string) {
			if candidate.runtimeEventQueue(owner) != nil {
				scriptUnequipByName(owner, name)
				return
			}
			stage(func() { scriptUnequipByName(owner, name) })
		})
		m["WithEquipment"] = reflect.ValueOf(func(name string, task func()) {
			if task == nil {
				return
			}
			if candidate.runtimeEventQueue(owner) != nil {
				scriptWithEquipment(owner, name, task)
				return
			}
			stage(func() {
				queueScriptCallbackOn(currentScriptEventQueue(owner), owner, "WithEquipment", func() {
					scriptWithEquipment(owner, name, task)
				})
			})
		})
		m["Bind"] = reflect.ValueOf(func(combo string, handler func(InputEvent)) Subscription {
			if candidate.claimBinding(combo, handler) {
				return subscribe(func() scriptRegistrationHandle { return scriptAddHotkeyFn(owner, combo, handler) })
			}
			return Subscription{}
		})
		m["AddToolbar"] = reflect.ValueOf(func(options scriptapi.ToolbarOptions) Subscription {
			if candidate.claimToolbar(options) {
				return subscribe(func() scriptRegistrationHandle {
					return scriptRegisterToolbar(owner, options, candidate.assets)
				})
			}
			return Subscription{}
		})
		m["Command"] = reflect.ValueOf(func(name string, handler func(string)) Subscription {
			if candidate.claimCommand(name, handler) {
				return subscribe(func() scriptRegistrationHandle { return scriptRegisterCommand(owner, name, handler) })
			}
			return Subscription{}
		})
		m["AddShortcut"] = reflect.ValueOf(func(short, full string) {
			stage(func() { scriptAddShortcut(owner, short, full) })
		})
		m["Print"] = reflect.ValueOf(func(msg string) { stage(func() { scriptConsole(msg) }) })
		m["ShowNotification"] = reflect.ValueOf(func(msg string) { stage(func() { scriptShowNotification(msg) }) })
		m["PlaySound"] = reflect.ValueOf(func(ids []uint16) {
			copyOfIDs := append([]uint16(nil), ids...)
			stage(func() { scriptPlaySound(copyOfIDs) })
		})
		m["Send"] = reflect.ValueOf(func(text string) {
			text = strings.TrimSpace(text)
			if candidate.runtimeEventQueue(owner) != nil {
				scriptCommand(owner, text)
				return
			}
			stage(func() { scriptCommand(owner, text) })
		})
		m["Store"] = reflect.ValueOf(func(key string, value any) { candidate.setStorage(owner, key, value) })
		m["LoadString"] = reflect.ValueOf(func(key, fallback string) string {
			return scriptStoredString(candidate.getStorage(owner, key), fallback)
		})
		m["LoadBool"] = reflect.ValueOf(func(key string, fallback bool) bool {
			return scriptStoredBool(candidate.getStorage(owner, key), fallback)
		})
		m["LoadInteger"] = reflect.ValueOf(func(key string, fallback int) int {
			return scriptStoredInteger(candidate.getStorage(owner, key), fallback)
		})
		m["LoadDecimal"] = reflect.ValueOf(func(key string, fallback float64) float64 {
			return scriptStoredDecimal(candidate.getStorage(owner, key), fallback)
		})
		m["LoadStrings"] = reflect.ValueOf(func(key string, fallback []string) []string {
			return scriptStoredStrings(candidate.getStorage(owner, key), fallback)
		})
		m["LoadJSON"] = reflect.ValueOf(func(key string, target any) bool { return scriptStoredJSON(candidate.getStorage(owner, key), target) })
		m["DeleteStored"] = reflect.ValueOf(func(key string) { candidate.deleteStorage(owner, key) })
		m["MigrateStorage"] = reflect.ValueOf(func(version int, migrate func(int)) {
			if version < 1 {
				panic("gt2.MigrateStorage version must be at least 1")
			}
			storedVersion := scriptStoredInteger(candidate.getStorage(owner, scriptStorageVersionKey), 0)
			if storedVersion > version {
				panic(fmt.Sprintf("stored data version %d is newer than this script's version %d", storedVersion, version))
			}
			if storedVersion == version {
				return
			}
			if migrate == nil {
				panic("gt2.MigrateStorage needs a migration function")
			}
			migrate(storedVersion)
			candidate.setStorage(owner, scriptStorageVersionKey, version)
		})
		m["SetInputText"] = reflect.ValueOf(func(text string) { stage(func() { scriptSetInputText(text) }) })
		m["OnChat"] = reflect.ValueOf(func(filter ChatFilter, handler func(ChatEvent)) Subscription {
			return subscribe(func() scriptRegistrationHandle { return scriptRegisterStructuredChat(owner, filter, handler) })
		})
		m["OnServerMessage"] = reflect.ValueOf(func(filter ServerMessageFilter, handler func(scriptapi.ServerMessage)) Subscription {
			return subscribe(func() scriptRegistrationHandle { return scriptRegisterServerMessage(owner, filter, handler) })
		})
		m["OnLogin"] = reflect.ValueOf(func(handler func(LifecycleEvent)) Subscription {
			return subscribe(func() scriptRegistrationHandle { return scriptRegisterLifecycle(owner, lifecycleLogin, handler) })
		})
		m["OnLogout"] = reflect.ValueOf(func(handler func(LifecycleEvent)) Subscription {
			return subscribe(func() scriptRegistrationHandle { return scriptRegisterLifecycle(owner, lifecycleLogout, handler) })
		})
		m["OnCharacterChange"] = reflect.ValueOf(func(handler func(LifecycleEvent)) Subscription {
			return subscribe(func() scriptRegistrationHandle {
				return scriptRegisterLifecycle(owner, lifecycleCharacterChange, handler)
			})
		})
		m["OnStop"] = reflect.ValueOf(func(handler func(LifecycleEvent)) Subscription {
			return subscribe(func() scriptRegistrationHandle { return scriptRegisterLifecycle(owner, lifecycleStop, handler) })
		})
		m["OnChange"] = reflect.ValueOf(func(kind string, handler func(ChangeEvent)) Subscription {
			return subscribe(func() scriptRegistrationHandle { return scriptRegisterChange(owner, kind, handler) })
		})
		m["WaitTicks"] = reflect.ValueOf(func(ticks int) {
			if eventQueue := candidate.runtimeEventQueue(owner); eventQueue != nil {
				scriptSleepTicks(owner, eventQueue, ticks)
			}
		})
		m["Wait"] = reflect.ValueOf(func(duration time.Duration) {
			if eventQueue := candidate.runtimeEventQueue(owner); eventQueue != nil {
				scriptWait(owner, eventQueue, duration)
			}
		})
		m["WaitForInventory"] = reflect.ValueOf(func(name string, present bool, timeout time.Duration) bool {
			return waitForScriptInventory(owner, candidate.runtimeEventQueue(owner), name, present, false, timeout)
		})
		m["WaitForEquipment"] = reflect.ValueOf(func(name string, equipped bool, timeout time.Duration) bool {
			return waitForScriptInventory(owner, candidate.runtimeEventQueue(owner), name, equipped, true, timeout)
		})
		// Simple world overlay drawing (top-left origin, world units)
		m["OverlayClear"] = reflect.ValueOf(func() { stage(func() { scriptOverlayClear(owner) }) })
		m["OverlayRect"] = reflect.ValueOf(func(x, y, w, h int, r, g, b, a uint8) {
			stage(func() { scriptOverlayRect(owner, x, y, w, h, r, g, b, a) })
		})
		m["OverlayText"] = reflect.ValueOf(func(x, y int, txt string, r, g, b, a uint8) {
			stage(func() { scriptOverlayText(owner, x, y, txt, r, g, b, a) })
		})
		m["OverlayImage"] = reflect.ValueOf(func(id uint16, x, y int) {
			stage(func() { scriptOverlayImage(owner, id, x, y) })
		})
		m["WorldSize"] = reflect.ValueOf(func() (int, int) { return gameAreaSizeX, gameAreaSizeY })
		m["ImageSize"] = reflect.ValueOf(func(id uint16) (int, int) {
			if clImages == nil {
				return 0, 0
			}
			w, h := clImages.Size(uint32(id))
			return w, h
		})
		m["Bool"] = reflect.ValueOf(func(option scriptapi.BoolOption) bool {
			entry, ok := makeTypedScriptConfigEntry(owner, option.Key, option.Label, option.Help, option.Scope, "bool", option.Default, option.OnChange, option.Validate, nil, 0, 0, 0)
			if !ok {
				return option.Default
			}
			stage(func() { scriptRegisterConfig(owner, entry) })
			return entry.Value.(bool)
		})
		m["Integer"] = reflect.ValueOf(func(option scriptapi.IntegerOption) int {
			entry, ok := makeTypedScriptConfigEntry(owner, option.Key, option.Label, option.Help, option.Scope, "int", option.Default, option.OnChange, option.Validate, nil, float64(option.Min), float64(option.Max), float64(option.Step))
			if !ok {
				return option.Default
			}
			stage(func() { scriptRegisterConfig(owner, entry) })
			return entry.Value.(int)
		})
		m["Decimal"] = reflect.ValueOf(func(option scriptapi.DecimalOption) float64 {
			entry, ok := makeTypedScriptConfigEntry(owner, option.Key, option.Label, option.Help, option.Scope, "float", option.Default, option.OnChange, option.Validate, nil, option.Min, option.Max, option.Step)
			if !ok {
				return option.Default
			}
			stage(func() { scriptRegisterConfig(owner, entry) })
			return entry.Value.(float64)
		})
		m["Text"] = reflect.ValueOf(func(option scriptapi.TextOption) string {
			entry, ok := makeTypedScriptConfigEntry(owner, option.Key, option.Label, option.Help, option.Scope, "text", option.Default, option.OnChange, option.Validate, nil, 0, 0, 0)
			if !ok {
				return option.Default
			}
			stage(func() { scriptRegisterConfig(owner, entry) })
			return entry.Value.(string)
		})
		m["Choice"] = reflect.ValueOf(func(option scriptapi.ChoiceOption) string {
			entry, ok := makeTypedScriptConfigEntry(owner, option.Key, option.Label, option.Help, option.Scope, "choice", option.Default, option.OnChange, nil, option.Choices, 0, 0, 0)
			if !ok {
				return option.Default
			}
			stage(func() { scriptRegisterConfig(owner, entry) })
			return entry.Value.(string)
		})
		m["KeyBinding"] = reflect.ValueOf(func(option scriptapi.KeyBindingOption) string {
			entry, ok := makeTypedScriptConfigEntry(owner, option.Key, option.Label, option.Help, option.Scope, "key", option.Default, option.OnChange, nil, nil, 0, 0, 0)
			if !ok {
				return option.Default
			}
			stage(func() { scriptRegisterConfig(owner, entry) })
			return entry.Value.(string)
		})
		m["ItemSelector"] = reflect.ValueOf(func(option scriptapi.ItemOption) string {
			entry, ok := makeTypedScriptConfigEntry(owner, option.Key, option.Label, option.Help, option.Scope, "item", option.Default, option.OnChange, nil, nil, 0, 0, 0)
			if !ok {
				return option.Default
			}
			stage(func() { scriptRegisterConfig(owner, entry) })
			return entry.Value.(string)
		})

		m["Repeat"] = reflect.ValueOf(func(interval time.Duration, fn func()) Timer {
			timer := newScriptTimer()
			if fn == nil || interval <= 0 {
				timer.Stop()
				return timer
			}
			stage(func() {
				timer.attach(startScriptRepeat(owner, currentScriptEventQueue(owner), interval, "Repeat", fn))
			})
			return timer
		})
		ex[pkg] = m
	}
	return ex
}

// script_library contains immutable examples. scripts/ is a separate Go module
// created for users' editor support and cannot be embedded by this module.
//
//go:embed script_library
var scriptScripts embed.FS

// userScriptsDir returns the preferred location for user-editable scripts.
// Scripts now live alongside the executable in a top-level "scripts" folder
// instead of under the data directory.
func userScriptsDir() string {
	if isWASM {
		return ""
	}
	exe, err := os.Executable()
	if err != nil {
		return "scripts"
	}
	return filepath.Join(filepath.Dir(exe), "scripts")
}

// scriptSearchDirs returns only the scripts/ folder next to the executable.
func scriptSearchDirs() []string {
	dir := userScriptsDir()
	if dir == "" {
		return nil
	}
	return []string{dir}
}

// ensureScriptsDir creates the user-owned scripts directory and installs the
// managed gt2 editor workspace. Bundled examples stay embedded until the user
// explicitly installs one from the library.
func ensureScriptsDir() {
	if isWASM {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Join(filepath.Dir(exe), "scripts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("create scripts dir: %v", err)
		return
	}
	if err := installScriptEditorSupport(dir); err != nil {
		log.Printf("install script editor support: %v", err)
	}
}

var scriptAllowedPkgs = []string{
	"bytes/bytes",
	"encoding/json/json",
	"errors/errors",
	"fmt/fmt",
	"math/big/big",
	"math/math",
	"math/rand/rand",
	"regexp/regexp",
	"sort/sort",
	"strconv/strconv",
	"strings/strings",
	"time/time",
	"unicode/utf8/utf8",
}

const scriptGoroutineLimit = 1024

var (
	scriptCallbackPanicHandler  func(owner, event string, recovered any, location string)
	scriptExecutionLimitHandler func(owner, event string, budgetLimited bool)
)

func init() {
	scriptCallbackPanicHandler = handleScriptCallbackPanic
	scriptExecutionLimitHandler = handleScriptExecutionLimit
	go scriptGoroutineWatchdog()
}

func scriptGoroutineWatchdog() {
	for {
		if runtime.NumGoroutine() > scriptGoroutineLimit {
			log.Printf("[script] goroutine limit exceeded; stopping all scripts")
			dispatchScriptControl(func() {
				consoleMessage("[script] goroutine limit exceeded; stopping scripts")
				stopAllscripts()
			})
			return
		}
		time.Sleep(time.Millisecond * 100)
	}
}

func restrictedStdlib() interp.Exports {
	restricted := interp.Exports{}
	for _, key := range scriptAllowedPkgs {
		if syms, ok := stdlib.Symbols[key]; ok {
			restricted[key] = syms
		}
	}
	return restricted
}

func scriptConsole(msg string) {
	consoleMessage(msg)
}

func scriptLogEvent(owner, ev, data string) {
	if !gs.scriptEventDebug {
		return
	}
	line := fmt.Sprintf("[%s] %s: %s", owner, ev, data)
	scriptDebugMu.Lock()
	scriptDebugLines = append(scriptDebugLines, line)
	if len(scriptDebugLines) > 200 {
		scriptDebugLines = scriptDebugLines[len(scriptDebugLines)-200:]
	}
	scriptDebugMu.Unlock()
	refreshscriptDebug()
}

func scriptShowNotification(msg string) {
	showNotification(msg)
}

func scriptIsDisabled(owner string) bool {
	scriptMu.RLock()
	disabled := scriptDisabled[owner]
	scriptMu.RUnlock()
	return disabled
}

// InputEvent describes a triggered key, mouse, modifier, chord, or wheel binding.
type InputEvent struct {
	Chord      string
	Key        string
	Button     string
	Modifiers  []string
	Ctrl       bool
	Alt        bool
	Shift      bool
	Meta       bool
	ScreenX    int
	ScreenY    int
	WorldX     int16
	WorldY     int16
	OnMobile   bool
	Mobile     Mobile
	PlayerName string
	SimpleName string
	decision   *inputEventDecision
}

type inputEventDecision struct {
	mu            sync.RWMutex
	continueInput bool
}

// Consume prevents the matching key, click, or wheel action from also being
// handled by the client.
func (event InputEvent) Consume() {
	if event.decision == nil {
		return
	}
	event.decision.mu.Lock()
	event.decision.continueInput = false
	event.decision.mu.Unlock()
}

// Pass allows the client to handle the matching input normally. This is the
// default, so most bindings do not need to call it.
func (event InputEvent) Pass() {
	if event.decision == nil {
		return
	}
	event.decision.mu.Lock()
	event.decision.continueInput = true
	event.decision.mu.Unlock()
}

// Continues reports whether the client will also handle the matching input.
func (event InputEvent) Continues() bool {
	if event.decision == nil {
		return true
	}
	event.decision.mu.RLock()
	defer event.decision.mu.RUnlock()
	return event.decision.continueInput
}

var (
	scriptHotkeyFnMu sync.RWMutex
	scriptHotkeyFns  = map[string]map[string]func(InputEvent) bool{}
)

// scriptAddHotkeyFn registers a function-based hotkey for a script.
// The hotkey appears in the "script Hotkeys" list and can be enabled/disabled
// like command-based hotkeys, but when pressed it will call the provided
// handler instead of emitting a slash command.
func scriptAddHotkeyFn(owner, combo string, handler func(InputEvent)) scriptRegistrationHandle {
	if scriptIsDisabled(owner) || handler == nil {
		return scriptRegistrationHandle{}
	}
	combo = strings.TrimSpace(combo)
	if combo == "" {
		return scriptRegistrationHandle{}
	}
	if bindingOwner, conflict := scriptBindingConflict(owner, combo); conflict {
		reportScriptBindingConflict(combo, bindingOwner)
		return scriptRegistrationHandle{}
	}
	// Ensure a visible toggleable hotkey entry exists for this script+combo.
	// Function-based hotkeys default to enabled on first add.
	hk := Hotkey{Name: "", Combo: combo, Script: owner, Disabled: false}
	scriptHotkeyMu.RLock()
	if m := scriptHotkeyEnabled[owner]; m != nil {
		if enabled, known := m[combo]; known {
			hk.Disabled = !enabled
		}
	}
	scriptHotkeyMu.RUnlock()
	hotkeysMu.Lock()
	for _, existing := range hotkeys {
		if existing.Script == owner && existing.Combo == combo {
			hotkeysMu.Unlock()
			refreshHotkeysList()
			saveHotkeys()
			return scriptRegistrationHandle{}
		}
	}
	var registration scriptRegistrationHandle
	registration = registerScriptResource(owner, func() {
		removeScriptHotkeyByHandle(registration)
	})
	hk.registration = registration
	hotkeys = append(hotkeys, hk)
	hotkeysMu.Unlock()

	// Remember handler only after the visible registration has been accepted.
	eventQueue := currentScriptEventQueue(owner)
	scriptHotkeyFnMu.Lock()
	m := scriptHotkeyFns[owner]
	if m == nil {
		m = map[string]func(InputEvent) bool{}
		scriptHotkeyFns[owner] = m
	}
	m[combo] = func(event InputEvent) bool {
		if !queueScriptCallbackWaitOn(eventQueue, owner, "Input", func() { handler(event) }) {
			return true
		}
		return event.Continues()
	}
	scriptHotkeyFnMu.Unlock()
	scriptHotkeyMu.Lock()
	stateMap := scriptHotkeyEnabled[owner]
	if stateMap == nil {
		stateMap = map[string]bool{}
		scriptHotkeyEnabled[owner] = stateMap
	}
	stateMap[combo] = !hk.Disabled
	scriptHotkeyMu.Unlock()
	refreshHotkeysList()
	saveHotkeys()
	scriptLogEvent(owner, "Registered binding", combo)
	return registration
}

func scriptGetHotkeyFn(owner, combo string) (func(InputEvent) bool, bool) {
	scriptHotkeyFnMu.RLock()
	defer scriptHotkeyFnMu.RUnlock()
	if m := scriptHotkeyFns[owner]; m != nil {
		if fn := m[combo]; fn != nil {
			return fn, true
		}
	}
	return nil, false
}

func reportScriptBindingConflict(combo, owner string) {
	msg := fmt.Sprintf("[script] binding conflict: %s already registered by %s", combo, owner)
	consoleMessage(msg)
	log.Print(msg)
}

// script command registries.
type scriptCommandHandler func(args string)

type structuredChatHandler struct {
	owner        string
	filter       ChatFilter
	fn           func(ChatEvent)
	queue        *scriptEventQueue
	registration scriptRegistrationHandle
}

type serverMessageHandler struct {
	owner        string
	filter       ServerMessageFilter
	fn           func(scriptapi.ServerMessage)
	queue        *scriptEventQueue
	registration scriptRegistrationHandle
}

type scriptLifecycleHandler struct {
	owner        string
	kind         string
	fn           func(LifecycleEvent)
	queue        *scriptEventQueue
	registration scriptRegistrationHandle
}

type scriptChangeHandler struct {
	owner        string
	kind         string
	fn           func(ChangeEvent)
	queue        *scriptEventQueue
	registration scriptRegistrationHandle
}

var (
	scriptCommands               = map[string]scriptCommandHandler{}
	scriptMu                     sync.RWMutex
	scriptNames                  = map[string]bool{}
	scriptDisplayNames           = map[string]string{}
	scriptAuthors                = map[string]string{}
	scriptCategories             = map[string]string{}
	scriptSubCategories          = map[string]string{}
	scriptDescriptions           = map[string]string{}
	scriptAPIVersions            = map[string]int{}
	scriptErrors                 = map[string]string{}
	scriptValidationResults      = map[string]string{}
	scriptReloadFailed           = map[string]bool{}
	scriptActiveSourceHashes     = map[string][sha256.Size]byte{}
	scriptInvalid                = map[string]bool{}
	scriptDisabled               = map[string]bool{}
	scriptEnabledFor             = map[string]scriptScope{}
	scriptPaths                  = map[string]string{}
	scriptPackages               = map[string]scriptInfo{}
	scriptTerminators            = map[string]func(){}
	scriptStructuredChatHandlers []structuredChatHandler
	scriptServerMessageHandlers  []serverMessageHandler
	scriptLifecycleHandlers      []scriptLifecycleHandler
	scriptChangeHandlers         []scriptChangeHandler
	chatHandlersMu               sync.RWMutex
	scriptCommandOwners          = map[string]string{}
	scriptSendHistory            = map[string][]time.Time{}
	scriptFileSnapshot           map[string]scriptFileState
	scriptModCheck               time.Time
	scriptRepeats                = map[string][]*scriptRepeatRegistration{}
	scriptTickWaiters            = map[string][]*tickWaiter{}
	scriptStopping               = map[string]bool{}

	// Per-script world overlay draw operations.
	scriptOverlayOps = map[string][]overlayOp{}
	overlayMu        sync.RWMutex

	scriptDebugLines []string
	scriptDebugMu    sync.Mutex
)

// overlayOp describes a simple draw command for the world overlay.
type overlayOp struct {
	kind       int // 0=rect, 1=text, 2=image
	x, y       int // world coordinates (top-left origin)
	w, h       int // for rect
	r, g, b, a uint8
	text       string // for text
	id         uint16 // for image (CL_Images pict ID)
}

type tickWaiter struct {
	remain int
	done   chan struct{}
}

type scriptRepeatRegistration struct {
	stop       chan struct{}
	eventQueue *scriptEventQueue
	event      string
	callback   func()
}

type scriptStateWaiter struct {
	queue  *scriptEventQueue
	signal chan struct{}
}

var scriptStateWaiters = map[string][]*scriptStateWaiter{}

func scriptSleepTicks(owner string, eventQueue *scriptEventQueue, ticks int) {
	if ticks <= 0 {
		return
	}
	w := &tickWaiter{remain: ticks, done: make(chan struct{}, 1)}
	scriptEventMu.Lock()
	if scriptEventQueues[owner] != eventQueue {
		scriptEventMu.Unlock()
		return
	}
	scriptMu.Lock()
	if scriptDisabled[owner] {
		scriptMu.Unlock()
		scriptEventMu.Unlock()
		return
	}
	scriptTickWaiters[owner] = append(scriptTickWaiters[owner], w)
	scriptMu.Unlock()
	scriptEventMu.Unlock()
	eventQueue.pauseExecution()
	defer eventQueue.resumeExecution()
	<-w.done
}

func scriptWait(owner string, eventQueue *scriptEventQueue, duration time.Duration) {
	if duration <= 0 || eventQueue == nil || !scriptEventQueueIsCurrent(owner, eventQueue) || scriptIsDisabled(owner) {
		return
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	eventQueue.pauseExecution()
	defer eventQueue.resumeExecution()
	if eventQueue.done == nil {
		<-timer.C
		return
	}
	select {
	case <-timer.C:
	case <-eventQueue.done:
	}
}

func waitForScriptInventory(owner string, eventQueue *scriptEventQueue, name string, want, equipmentOnly bool, timeout time.Duration) bool {
	name = strings.TrimSpace(name)
	if name == "" || timeout <= 0 || eventQueue == nil || !scriptEventQueueIsCurrent(owner, eventQueue) || scriptIsDisabled(owner) {
		return false
	}
	matches := func() bool {
		found := false
		for _, item := range getInventory() {
			if strings.EqualFold(item.Name, name) || strings.EqualFold(item.Base, name) {
				found = !equipmentOnly || item.Equipped
				if found {
					break
				}
			}
		}
		return found == want
	}
	if matches() {
		return true
	}
	waiter := &scriptStateWaiter{queue: eventQueue, signal: make(chan struct{}, 1)}
	scriptMu.Lock()
	scriptStateWaiters[owner] = append(scriptStateWaiters[owner], waiter)
	scriptMu.Unlock()
	defer func() {
		scriptMu.Lock()
		list := scriptStateWaiters[owner]
		for i, candidate := range list {
			if candidate == waiter {
				list = append(list[:i], list[i+1:]...)
				break
			}
		}
		if len(list) == 0 {
			delete(scriptStateWaiters, owner)
		} else {
			scriptStateWaiters[owner] = list
		}
		scriptMu.Unlock()
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	eventQueue.pauseExecution()
	defer eventQueue.resumeExecution()
	for {
		select {
		case <-waiter.signal:
			if matches() {
				return true
			}
		case <-timer.C:
			return false
		case <-eventQueue.done:
			return false
		}
	}
}

func notifyScriptStateWaiters() {
	scriptMu.Lock()
	for _, waiters := range scriptStateWaiters {
		for _, waiter := range waiters {
			if waiter == nil {
				continue
			}
			select {
			case waiter.signal <- struct{}{}:
			default:
			}
		}
	}
	scriptMu.Unlock()
}

func startScriptRepeat(owner string, eventQueue *scriptEventQueue, interval time.Duration, event string, fn func()) func() {
	if eventQueue == nil || interval <= 0 || fn == nil {
		return nil
	}
	repeat := &scriptRepeatRegistration{stop: make(chan struct{}), eventQueue: eventQueue, event: event, callback: fn}
	scriptMu.Lock()
	if scriptDisabled[owner] {
		scriptMu.Unlock()
		return nil
	}
	scriptRepeats[owner] = append(scriptRepeats[owner], repeat)
	scriptMu.Unlock()
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				queueScriptCallbackOn(repeat.eventQueue, owner, repeat.event, repeat.callback)
			case <-repeat.stop:
				return
			}
		}
	}()
	return func() { stopScriptRepeat(owner, repeat) }
}

func stopScriptRepeat(owner string, repeat *scriptRepeatRegistration) {
	if repeat == nil {
		return
	}
	scriptMu.Lock()
	list := scriptRepeats[owner]
	for i, candidate := range list {
		if candidate != repeat {
			continue
		}
		close(repeat.stop)
		list = append(list[:i], list[i+1:]...)
		if len(list) == 0 {
			delete(scriptRepeats, owner)
		} else {
			scriptRepeats[owner] = list
		}
		break
	}
	scriptMu.Unlock()
}

func scriptAdvanceTick() {
	scriptMu.Lock()
	for owner, list := range scriptTickWaiters {
		n := 0
		for _, w := range list {
			if w == nil {
				continue
			}
			w.remain--
			if w.remain <= 0 {
				select {
				case w.done <- struct{}{}:
				default:
				}
			} else {
				list[n] = w
				n++
			}
		}
		if n == 0 {
			delete(scriptTickWaiters, owner)
		} else {
			scriptTickWaiters[owner] = list[:n]
		}
	}
	scriptMu.Unlock()
}

const (
	minscriptMetaLen = 2
	maxscriptMetaLen = 40
)

func invalidscriptValue(s string) bool {
	l := len(s)
	return l < minscriptMetaLen || l > maxscriptMetaLen
}

// scriptRegisterCommand lets scripts handle a local slash command like
// "/example". The name should be without the leading slash and will be
// matched case-insensitively.
func scriptRegisterCommand(owner, name string, handler scriptCommandHandler) scriptRegistrationHandle {
	if name == "" || handler == nil {
		return scriptRegistrationHandle{}
	}
	if scriptIsDisabled(owner) {
		return scriptRegistrationHandle{}
	}
	key := normalizeScriptCommand(name)
	if key == "" {
		return scriptRegistrationHandle{}
	}
	scriptMu.Lock()
	if _, exists := scriptCommands[key]; exists {
		scriptMu.Unlock()
		msg := fmt.Sprintf("[script] command conflict: /%s already registered", key)
		consoleMessage(msg)
		log.Print(msg)
		return scriptRegistrationHandle{}
	}
	eventQueue := currentScriptEventQueue(owner)
	scriptCommands[key] = func(args string) {
		queueScriptCallbackOn(eventQueue, owner, "Command", func() { handler(args) })
	}
	scriptCommandOwners[key] = owner
	scriptMu.Unlock()
	registration := registerScriptResource(owner, func() {
		scriptMu.Lock()
		if scriptCommandOwners[key] == owner {
			delete(scriptCommands, key)
			delete(scriptCommandOwners, key)
		}
		scriptMu.Unlock()
	})
	scriptLogEvent(owner, "Registered command", "/"+key)
	return registration
}

// scriptCommand is the single path for script-generated server commands.
func scriptCommand(owner, cmd string) bool {
	if scriptIsDisabled(owner) {
		return false
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		reportScriptCommandError(owner, "command rejected: empty command")
		return false
	}
	if recordscriptSend(owner) {
		reportScriptCommandError(owner, "command rejected: rate limit exceeded")
		return false
	}
	enqueueCommand(cmd)
	return true
}

func reportScriptCommandError(owner, message string) {
	consoleMessage("[script:" + scriptDisplayName(owner) + "] " + message)
}

type preparedScript struct {
	candidate   *scriptCandidate
	initialize  func()
	terminate   func()
	interpreter *interp.Interpreter
	diagnostics *scriptDiagnostics
}

func compileScriptSource(owner string, src []byte, restricted interp.Exports) (*preparedScript, error) {
	return compileScriptSourceWithAssets(owner, src, restricted, nil)
}

func compileScriptSourceWithAssets(owner string, src []byte, restricted interp.Exports, assets *scriptAssetSource) (*preparedScript, error) {
	if err := checkScriptSourceRequirements(src); err != nil {
		return nil, err
	}
	candidate := &scriptCandidate{assets: assets}
	diagnostics := &scriptDiagnostics{}
	i := interp.New(interp.Options{Stderr: diagnostics})
	if len(restricted) > 0 {
		i.Use(restricted)
	}
	i.Use(exportsForScriptCandidate(owner, candidate))
	// Strip build tags like //go:build which are for the Go toolchain only.
	src = stripGoBuildDirectives(src)
	if _, err := i.Eval(string(src)); err != nil {
		candidate.discard()
		return nil, err
	}
	prepared := &preparedScript{candidate: candidate, interpreter: i, diagnostics: diagnostics}
	if v, err := i.Eval("Terminate"); err == nil {
		if fn, ok := v.Interface().(func()); ok {
			prepared.terminate = func() { candidate.callTerminate(fn) }
		}
	}
	v, err := i.Eval("Init")
	if err != nil {
		disposePreparedScript(prepared)
		return nil, fmt.Errorf("missing required func Init()")
	}
	fn, ok := v.Interface().(func())
	if !ok {
		disposePreparedScript(prepared)
		return nil, fmt.Errorf("Init must have the signature func Init()")
	}
	prepared.initialize = fn
	return prepared, nil
}

func initializePreparedScript(prepared *preparedScript) error {
	if prepared == nil || prepared.initialize == nil {
		return nil
	}
	prepared.diagnostics.clear()
	if err := callScriptLifecycle("Init", prepared.initialize, prepared.interpreter); err != nil {
		if location := prepared.diagnostics.panicLocation(); location != "" {
			return fmt.Errorf("%w at %s", err, location)
		}
		return err
	}
	return nil
}

func disposePreparedScript(prepared *preparedScript) {
	if prepared == nil {
		return
	}
	prepared.candidate.discard()
	interruptScriptInterpreter(prepared.interpreter)
}

func prepareScriptSource(owner string, src []byte, restricted interp.Exports) (*preparedScript, error) {
	return prepareScriptSourceWithAssets(owner, src, restricted, nil)
}

func prepareScriptSourceWithAssets(owner string, src []byte, restricted interp.Exports, assets *scriptAssetSource) (*preparedScript, error) {
	prepared, err := compileScriptSourceWithAssets(owner, src, restricted, assets)
	if err != nil {
		return nil, err
	}
	if err := initializePreparedScript(prepared); err != nil {
		disposePreparedScript(prepared)
		return nil, err
	}
	return prepared, nil
}

func callScriptLifecycle(event string, fn func(), interpreters ...*interp.Interpreter) error {
	result := make(chan error, 1)
	go func() {
		var err error
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("%s panic: %v", event, recovered)
			}
			result <- err
		}()
		fn()
	}()
	timer := time.NewTimer(scriptCallbackTimeLimit)
	defer timer.Stop()
	select {
	case err := <-result:
		return err
	case <-timer.C:
		if len(interpreters) > 0 {
			interruptScriptInterpreter(interpreters[0])
		}
		return fmt.Errorf("%s exceeded the callback time limit", event)
	}
}

func runScriptCallback(owner, event string, fn func()) (ok bool) {
	if fn == nil || scriptIsDisabled(owner) {
		return false
	}
	ok = true
	defer func() {
		if recovered := recover(); recovered != nil {
			ok = false
			if scriptCallbackPanicHandler != nil {
				scriptCallbackPanicHandler(owner, event, recovered, scriptCallbackSourceLocation(owner))
			}
		}
	}()
	fn()
	return ok
}

func scriptCallbackSourceLocation(owner string) string {
	queue := currentScriptEventQueue(owner)
	if queue == nil {
		return ""
	}
	return queue.diagnostics.panicLocation()
}

func scriptDisplayName(owner string) string {
	scriptMu.RLock()
	name := scriptDisplayNames[owner]
	scriptMu.RUnlock()
	if name == "" {
		return owner
	}
	return name
}

func formatScriptError(path string, err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if path != "" {
		message = strings.ReplaceAll(message, "_.go:", path+":")
		if !strings.Contains(message, path) {
			message = path + ": " + message
		}
	}
	return message
}

func recordScriptError(owner, message string, reloadFailed bool) {
	scriptMu.Lock()
	scriptErrors[owner] = message
	scriptReloadFailed[owner] = reloadFailed
	scriptMu.Unlock()
}

func handleScriptCallbackPanic(owner, event string, recovered any, location string) {
	name := scriptDisplayName(owner)
	where := ""
	if location != "" {
		where = " at " + location
	}
	msg := fmt.Sprintf("[script:%s] %s callback panic%s: %v", name, event, where, recovered)
	scriptMu.RLock()
	path := scriptPaths[owner]
	scriptMu.RUnlock()
	errorLocation := location
	if path != "" && location != "" {
		errorLocation = path + ":" + location
	}
	errorWhere := ""
	if errorLocation != "" {
		errorWhere = " at " + errorLocation
	}
	recordScriptError(owner, fmt.Sprintf("%s callback panic%s: %v", event, errorWhere, recovered), false)
	log.Print(msg)
	scriptMu.Lock()
	stopping := scriptStopping[owner]
	alreadyDisabled := scriptDisabled[owner]
	scriptDisabled[owner] = true
	scriptMu.Unlock()
	if stopping {
		dispatchScriptControl(func() { consoleMessage(msg) })
		return
	}
	if alreadyDisabled {
		return
	}
	dispatchScriptControl(func() {
		consoleMessage(msg)
		disablescript(owner, "panic in "+event+" callback")
	})
}

func handleScriptExecutionLimit(owner, event string, budgetLimited bool) {
	limitName := "callback time limit"
	reason := "execution time limit"
	if budgetLimited {
		limitName = "script execution budget"
		reason = "execution budget"
	}
	msg := fmt.Sprintf("[script:%s] %s callback exceeded the %s", owner, event, limitName)
	recordScriptError(owner, msg, false)
	log.Print(msg)
	scriptMu.Lock()
	alreadyDisabled := scriptDisabled[owner]
	scriptDisabled[owner] = true
	scriptMu.Unlock()
	if alreadyDisabled {
		return
	}
	dispatchScriptControl(func() {
		consoleMessage(msg)
		disablescript(owner, reason+" exceeded")
	})
}

func scriptCandidateConflict(owner string, candidate *scriptCandidate) error {
	candidate.mu.Lock()
	commands := make([]string, 0, len(candidate.commands))
	for command := range candidate.commands {
		commands = append(commands, command)
	}
	sort.Strings(commands)
	bindings := append([]string(nil), candidate.bindings...)
	conflicts := append([]string(nil), candidate.conflicts...)
	candidate.mu.Unlock()
	if len(conflicts) > 0 {
		return fmt.Errorf("%s", conflicts[0])
	}

	scriptMu.RLock()
	for _, command := range commands {
		if existingOwner, exists := scriptCommandOwners[command]; exists && existingOwner != owner {
			scriptMu.RUnlock()
			return fmt.Errorf("duplicate command /%s already owned by %s", command, existingOwner)
		}
	}
	scriptMu.RUnlock()

	for _, combo := range bindings {
		if bindingOwner, exists := scriptBindingConflict(owner, combo); exists {
			return fmt.Errorf("duplicate binding %s already owned by %s", combo, bindingOwner)
		}
	}
	return nil
}

func scriptBindingConflict(owner, combo string) (string, bool) {
	hotkeysMu.RLock()
	defer hotkeysMu.RUnlock()
	for _, existing := range hotkeys {
		if existing.Script != owner && sameCombo(existing.Combo, combo) {
			bindingOwner := existing.Script
			if bindingOwner == "" {
				bindingOwner = "global hotkeys"
			}
			return bindingOwner, true
		}
	}
	return "", false
}

func scriptIsRunning(owner string) bool {
	scriptMu.RLock()
	disabled, known := scriptDisabled[owner]
	scriptMu.RUnlock()
	return known && !disabled
}

func activatePreparedScript(owner string, prepared *preparedScript) {
	scriptMu.Lock()
	scriptDisabled[owner] = false
	delete(scriptErrors, owner)
	delete(scriptReloadFailed, owner)
	if prepared.terminate != nil {
		scriptTerminators[owner] = prepared.terminate
	}
	scriptMu.Unlock()
	eventQueue := startScriptEventQueue(owner, prepared.interpreter)
	eventQueue.diagnostics = prepared.diagnostics
	prepared.candidate.activate(eventQueue)
	flushscriptStore(owner)
}

func loadscriptSource(owner, name, path string, src []byte, restricted interp.Exports) bool {
	return loadscriptPackageSource(owner, name, path, src, restricted, nil, sha256.Sum256(src))
}

func loadscriptPackageSource(owner, name, path string, src []byte, restricted interp.Exports, assets *scriptAssetSource, fingerprint [sha256.Size]byte) bool {
	wasRunning := scriptIsRunning(owner)
	prepared, err := prepareScriptSourceWithAssets(owner, src, restricted, assets)
	if err == nil {
		err = scriptCandidateConflict(owner, prepared.candidate)
		if err != nil {
			disposePreparedScript(prepared)
		}
	}
	if err != nil {
		log.Printf("script %s: %v", path, err)
		message := formatScriptError(path, err)
		recordScriptError(owner, message, wasRunning)
		if wasRunning {
			consoleMessage("[script] reload error for " + path + ": " + message)
			refreshscriptsWindow()
		} else {
			consoleMessage("[script] load error for " + path + ": " + message)
			disablescript(owner, "load error")
		}
		return false
	}
	if wasRunning {
		disablescript(owner, "reloaded")
	}
	activatePreparedScript(owner, prepared)
	scriptMu.Lock()
	if scriptActiveSourceHashes == nil {
		scriptActiveSourceHashes = map[string][sha256.Size]byte{}
	}
	scriptActiveSourceHashes[owner] = fingerprint
	scriptMu.Unlock()
	log.Printf("loaded script %s", path)
	consoleMessage("[script] loaded: " + name)
	return true
}

// stripGoBuildDirectives removes leading build constraints (//go:build, // +build)
// which are meaningful to the Go toolchain but can confuse the interpreter.
func stripGoBuildDirectives(src []byte) []byte {
	lines := strings.Split(string(src), "\n")
	out := make([]string, 0, len(lines))
	i := 0
	// Skip initial build constraint lines and following blank lines until package clause
	for i < len(lines) {
		l := strings.TrimSpace(lines[i])
		if strings.HasPrefix(l, "package ") {
			break
		}
		if strings.HasPrefix(l, "//go:build") || strings.HasPrefix(l, "// +build") || l == "" {
			i++
			continue
		}
		// Any other pre-package content: keep it
		break
	}
	if i > 0 {
		out = append(out, lines[i:]...)
		return []byte(strings.Join(out, "\n"))
	}
	return src
}

func enablescript(owner string) {
	scriptMu.RLock()
	info, ok := scriptPackages[owner]
	scriptMu.RUnlock()
	if !ok {
		info, ok = scanscripts(scriptSearchDirs(), nil)[owner]
	}
	if !ok {
		return
	}
	loadscriptPackageSource(owner, info.name, info.path, info.src, restrictedStdlib(), info.assets, info.fingerprint)
	settingsDirty = true
	saveSettings()
	refreshscriptsWindow()
}

func recordscriptSend(owner string) bool {
	if !gs.ScriptSpamKill {
		return false
	}
	now := time.Now()
	cutoff := now.Add(-5 * time.Second)
	scriptMu.Lock()
	times := scriptSendHistory[owner]
	n := 0
	for _, t := range times {
		if t.After(cutoff) {
			times[n] = t
			n++
		}
	}
	times = times[:n]
	times = append(times, now)
	scriptSendHistory[owner] = times
	count := len(times)
	scriptMu.Unlock()
	if count > 30 {
		disablescript(owner, "sent too many lines")
		return true
	}
	return false
}

type deactivatedScript struct {
	terminate  func()
	eventQueue *scriptEventQueue
}

func deactivateScript(owner, reason string) deactivatedScript {
	scriptMu.Lock()
	scriptDisabled[owner] = true
	if reason != "disabled for this character" && reason != "reloaded" {
		delete(scriptEnabledFor, owner)
	}
	term := scriptTerminators[owner]
	delete(scriptTerminators, owner)
	scriptMu.Unlock()
	eventQueue := stopScriptEventQueue(owner)
	return deactivatedScript{terminate: term, eventQueue: eventQueue}
}

func terminateDeactivatedScript(owner string, deactivated deactivatedScript) {
	if deactivated.terminate != nil {
		var interpreter *interp.Interpreter
		if deactivated.eventQueue != nil {
			interpreter = deactivated.eventQueue.interpreter
			deactivated.eventQueue.diagnostics.clear()
		}
		if err := callScriptLifecycle("Terminate", deactivated.terminate, interpreter); err != nil {
			log.Printf("script %s: %v", owner, err)
			location := ""
			if deactivated.eventQueue != nil {
				location = deactivated.eventQueue.diagnostics.panicLocation()
			}
			where := ""
			if location != "" {
				where = " at " + location
			}
			consoleMessage("[script:" + scriptDisplayName(owner) + "] Terminate error" + where + ": " + err.Error())
		}
	}
}

func disposeScriptResources(owner, reason string, eventQueue *scriptEventQueue) {
	cancelScriptDispatch(owner)
	releaseScriptRegistrations(eventQueue)
	if scriptConfigWin != nil && scriptConfigOwner == owner {
		scriptConfigWin.Close()
		scriptConfigWin = nil
		scriptConfigOwner = ""
	}
	// Clear overlay ops
	overlayMu.Lock()
	delete(scriptOverlayOps, owner)
	overlayMu.Unlock()
	// Stop repeating timers and tick waiters for this script.
	scriptMu.Lock()
	if repeats := scriptRepeats[owner]; len(repeats) > 0 {
		for _, repeat := range repeats {
			if repeat != nil && repeat.stop != nil {
				close(repeat.stop)
			}
		}
		delete(scriptRepeats, owner)
	}
	if waits := scriptTickWaiters[owner]; len(waits) > 0 {
		for _, w := range waits {
			if w != nil {
				select {
				case w.done <- struct{}{}:
				default:
				}
			}
		}
		delete(scriptTickWaiters, owner)
	}
	scriptMu.Unlock()
	scriptMu.Lock()
	delete(scriptSendHistory, owner)
	disp := scriptDisplayNames[owner]
	scriptMu.Unlock()
	if disp == "" {
		disp = owner
	}
	flushscriptStore(owner)
	consoleMessage("[script:" + disp + "] stopped: " + reason)
	settingsDirty = true
	saveSettings()
	refreshscriptsWindow()
}

func disablescript(owner, reason string) {
	scriptMu.Lock()
	if scriptStopping[owner] {
		scriptMu.Unlock()
		return
	}
	scriptStopping[owner] = true
	scriptMu.Unlock()
	runScriptStopHandlers(owner, reason)
	deactivated := deactivateScript(owner, reason)
	terminateDeactivatedScript(owner, deactivated)
	disposeScriptResources(owner, reason, deactivated.eventQueue)
	scriptMu.Lock()
	delete(scriptStopping, owner)
	scriptMu.Unlock()
}

func stopAllscripts() {
	scriptMu.RLock()
	owners := make([]string, 0, len(scriptDisplayNames))
	for o := range scriptDisplayNames {
		if !scriptDisabled[o] {
			owners = append(owners, o)
		}
	}
	scriptMu.RUnlock()
	sort.Strings(owners)
	for _, o := range owners {
		disablescript(o, "stopped by user")
	}
	if len(owners) > 0 {
		clearCommands()
		consoleMessage("[script] all scripts stopped")
	}
}

func applyEnabledScripts() {
	scriptMu.RLock()
	owners := make([]string, 0, len(scriptDisplayNames))
	for o := range scriptDisplayNames {
		owners = append(owners, o)
	}
	scriptMu.RUnlock()
	sort.Strings(owners)
	for _, o := range owners {
		scriptMu.RLock()
		scope := scriptEnabledFor[o]
		disabled := scriptDisabled[o]
		invalid := scriptInvalid[o]
		scriptMu.RUnlock()
		if invalid {
			// A running script may have become invalid after an edit. Keep its
			// last working interpreter alive until a valid replacement is ready.
			continue
		}
		// Enable when set to all, or when the scope includes the active
		// character. If not logged in, fall back to LastCharacter.
		effChar := playerName
		if effChar == "" {
			effChar = gs.LastCharacter
		}
		shouldEnable := scope.enablesFor(effChar)
		if disabled && shouldEnable {
			enablescript(o)
		} else if !disabled && !shouldEnable {
			disablescript(o, "disabled for this character")
		} else {
			scriptMu.Lock()
			scriptDisabled[o] = !shouldEnable
			scriptMu.Unlock()
		}
	}
}

func setscriptEnabled(owner string, char, all bool) {
	scriptMu.Lock()
	if scriptInvalid[owner] {
		scriptMu.Unlock()
		return
	}
	s := scriptEnabledFor[owner]
	if all {
		s.All = true
		s.Chars = nil
	} else if char {
		effChar := playerName
		if effChar == "" {
			effChar = gs.LastCharacter
		}
		if effChar != "" {
			s.All = false
			s.addChar(effChar)
		}
	} else {
		effChar := playerName
		if effChar == "" {
			effChar = gs.LastCharacter
		}
		if effChar != "" {
			s.removeChar(effChar)
		} else {
			s = scriptScope{}
		}
	}
	if s.empty() {
		delete(scriptEnabledFor, owner)
	} else {
		scriptEnabledFor[owner] = s
	}
	scriptMu.Unlock()
	applyEnabledScripts()
	saveSettings()
	refreshscriptsWindow()
}

// clearscriptScope removes all enablement for a script (no all, no characters)
// and refreshes apply/save/UI. Used by the UI when unchecking the "All" box
// to explicitly stop a script regardless of any per-character flags.
func clearscriptScope(owner string) {
	scriptMu.Lock()
	delete(scriptEnabledFor, owner)
	// Stop repeating timers for this script.
	if repeats := scriptRepeats[owner]; len(repeats) > 0 {
		for _, repeat := range repeats {
			if repeat != nil && repeat.stop != nil {
				close(repeat.stop)
			}
		}
		delete(scriptRepeats, owner)
	}
	scriptMu.Unlock()
	applyEnabledScripts()
	saveSettings()
	refreshscriptsWindow()
}

func scriptPlayers() []scriptapi.Player {
	ps := getPlayers()
	out := make([]scriptapi.Player, len(ps))
	for index, player := range ps {
		out[index] = scriptPlayerSnapshot(player)
	}
	return out
}

func scriptInventory() []InventoryItem {
	return getInventory()
}

func scriptInputText() string {
	inputMu.Lock()
	txt := string(inputText)
	inputMu.Unlock()
	return txt
}

func scriptSetInputText(text string) {
	inputMu.Lock()
	inputText = []rune(text)
	inputActive = true
	inputPos = len(inputText)
	inputMu.Unlock()
}

// scriptEquipByName equips the first inventory item whose name matches the
// provided name (case-insensitive). If the item is already equipped, it skips.
func scriptEquipByName(owner, name string) {
	targetName := strings.ToLower(strings.TrimSpace(name))
	if targetName == "" {
		reportScriptCommandError(owner, "equip target not found: empty name")
		return
	}
	items := getInventory()
	var id uint16
	idx := -1
	found := false
	for _, it := range items {
		if strings.ToLower(it.Name) != targetName {
			continue
		}
		found = true
		// If any matching item is already equipped, skip as redundant.
		if it.Equipped {
			n := it.Name
			if n == "" {
				n = targetName
			}
			consoleMessage(n + " already equipped, skipping")
			return
		}
		// Prefer the first match; use its ID and server-provided index.
		id = it.ID
		if idx < 0 {
			idx = it.IDIndex
		}
		// Do not break; if there are multiple matches, the first branch sets id/idx
		// and we continue to see if an equipped instance exists to early-out.
	}
	if !found {
		reportScriptCommandError(owner, "equip target not found: "+name)
		return
	}
	if scriptCommand(owner, formatEquipCommand(id, idx)) {
		equipInventoryItem(id, idx, true)
	}
}

// scriptUnequipByName unequips an item by name (case-insensitive). If multiple
// items share the name, it unequips any equipped instance.
func scriptUnequipByName(owner, name string) {
	targetName := strings.ToLower(strings.TrimSpace(name))
	if targetName == "" {
		reportScriptCommandError(owner, "unequip target not found: empty name")
		return
	}
	items := getInventory()
	var id uint16
	equipped := false
	for _, it := range items {
		if strings.ToLower(it.Name) != targetName {
			continue
		}
		if it.Equipped {
			id = it.ID
			equipped = true
			break
		}
		// Remember an ID even if not equipped yet; we still require equipped=true
		// to proceed, matching previous Unequip behavior.
		if id == 0 {
			id = it.ID
		}
	}
	if !equipped {
		reportScriptCommandError(owner, "unequip target not equipped: "+name)
		return
	}
	if scriptCommand(owner, fmt.Sprintf("/unequip %d", id)) {
		equipInventoryItem(id, -1, false)
	}
}

type scriptInventoryKey struct {
	id  uint16
	idx int
}

// scriptWithEquipment equips one item for task and restores the equipment that
// previously occupied the same slot even when task panics.
func scriptWithEquipment(owner, name string, task func()) {
	if task == nil || scriptIsDisabled(owner) {
		return
	}
	targetName := strings.ToLower(strings.TrimSpace(name))
	items := getInventory()
	var target InventoryItem
	found := false
	for _, item := range items {
		if strings.ToLower(item.Name) == targetName || strings.ToLower(item.Base) == targetName {
			target, found = item, true
			break
		}
	}
	if !found {
		reportScriptCommandError(owner, "equip target not found: "+name)
		return
	}
	targetSlot := -1
	if clImages != nil {
		targetSlot = clImages.ItemSlot(uint32(target.ID))
	}
	inSlot := func(item InventoryItem) bool {
		if targetSlot >= 0 && clImages != nil {
			return clImages.ItemSlot(uint32(item.ID)) == targetSlot
		}
		return item.ID == target.ID && item.IDIndex == target.IDIndex
	}
	prior := map[scriptInventoryKey]InventoryItem{}
	for _, item := range items {
		if item.Equipped && inSlot(item) {
			prior[scriptInventoryKey{id: item.ID, idx: item.IDIndex}] = item
		}
	}
	if !target.Equipped {
		if !scriptCommand(owner, formatEquipCommand(target.ID, target.IDIndex)) {
			return
		}
		equipInventoryItem(target.ID, target.IDIndex, true)
	}
	defer func() {
		current := getInventory()
		for _, item := range current {
			if !item.Equipped || !inSlot(item) {
				continue
			}
			if _, existed := prior[scriptInventoryKey{id: item.ID, idx: item.IDIndex}]; !existed {
				if scriptCommand(owner, fmt.Sprintf("/unequip %d", item.ID)) {
					equipInventoryItem(item.ID, item.IDIndex, false)
				}
			}
		}
		current = getInventory()
		for key, item := range prior {
			equipped := false
			for _, candidate := range current {
				if candidate.ID == key.id && candidate.IDIndex == key.idx && candidate.Equipped {
					equipped = true
					break
				}
			}
			if !equipped && scriptCommand(owner, formatEquipCommand(item.ID, item.IDIndex)) {
				equipInventoryItem(item.ID, item.IDIndex, true)
			}
		}
	}()
	task()
}

// Chat trigger kinds for filtering messages by source.
const (
	ChatAny      = 1 << iota // match any chat message
	ChatPlayer               // message from a known player (not NPC)
	ChatNPC                  // message from a known NPC
	ChatCreature             // message from an unknown/non-player speaker
	ChatSelf                 // message from yourself
	ChatOther                // message not from yourself
)

const (
	ChatTypeSay     = "say"
	ChatTypeYell    = "yell"
	ChatTypeWhisper = "whisper"
	ChatTypeAsk     = "ask"
	ChatTypeExclaim = "exclaim"
	ChatTypeThinkTo = "think-to"
	ChatTypeEmote   = "emote"
)

// ChatFilter selects structured chat events. Zero values match all chat.
type ChatFilter struct {
	Contains string
	Speaker  string
	Kinds    int
	Type     string
}

// ChatEvent contains parsed chat data while retaining the original line.
type ChatEvent struct {
	Speaker string
	Message string
	Raw     string
	Kinds   int
	Type    string
}

type ServerMessageFilter struct {
	Contains string
	Type     string
}

const (
	lifecycleLogin           = "login"
	lifecycleLogout          = "logout"
	lifecycleCharacterChange = "character-change"
	lifecycleStop            = "stop"
)

type LifecycleEvent struct {
	Type              string
	Character         string
	PreviousCharacter string
	Reason            string
}

const (
	ChangeInventory      = "inventory"
	ChangeEquipment      = "equipment"
	ChangeSelectedPlayer = "selected-player"
	ChangeSelectedItem   = "selected-item"
	ChangeVitals         = "vitals"
	ChangeWorld          = "world"
	ChangeLocation       = "location"
)

type ChangeEvent struct {
	Type            string
	Inventory       []InventoryItem
	Equipment       []InventoryItem
	SelectedPlayer  string
	SelectedItem    InventoryItem
	HasSelectedItem bool
	Health          int
	HealthMax       int
	Spirit          int
	SpiritMax       int
	Balance         int
	BalanceMax      int
	Location        string
	WorldGeneration uint64
}

type scriptChangeSnapshot struct {
	initialized         bool
	inventory           []InventoryItem
	equipment           []InventoryItem
	selectedPlayer      string
	selectedItem        InventoryItem
	hasSelectedItem     bool
	health, healthMax   int
	spirit, spiritMax   int
	balance, balanceMax int
	location            string
	worldGeneration     uint64
}

var (
	scriptChangeMu   sync.Mutex
	scriptChanges    scriptChangeSnapshot
	scriptLocationMu sync.RWMutex
	scriptLocation   string
)

var (
	scriptSessionMu        sync.Mutex
	scriptSessionCharacter string
	scriptSessionActive    bool
)

var (
	scriptLatestServerMessageMu       sync.RWMutex
	scriptLatestServerMessageSnapshot scriptapi.ServerMessage
	scriptLatestServerMessageSequence uint64
	scriptHasLatestServerMessage      bool
)

func scriptLatestServerMessage() (scriptapi.ServerMessage, bool) {
	scriptLatestServerMessageMu.RLock()
	defer scriptLatestServerMessageMu.RUnlock()
	return scriptLatestServerMessageSnapshot, scriptHasLatestServerMessage
}

func clearScriptLatestServerMessage() {
	scriptLatestServerMessageMu.Lock()
	scriptLatestServerMessageSnapshot = scriptapi.ServerMessage{}
	scriptLatestServerMessageSequence = 0
	scriptHasLatestServerMessage = false
	scriptLatestServerMessageMu.Unlock()
}

func scriptRegisterStructuredChat(owner string, filter ChatFilter, fn func(ChatEvent)) scriptRegistrationHandle {
	if scriptIsDisabled(owner) || fn == nil {
		return scriptRegistrationHandle{}
	}
	var registration scriptRegistrationHandle
	registration = registerScriptResource(owner, func() {
		chatHandlersMu.Lock()
		for i := len(scriptStructuredChatHandlers) - 1; i >= 0; i-- {
			if scriptStructuredChatHandlers[i].registration == registration {
				scriptStructuredChatHandlers = append(scriptStructuredChatHandlers[:i], scriptStructuredChatHandlers[i+1:]...)
			}
		}
		chatHandlersMu.Unlock()
	})
	chatHandlersMu.Lock()
	scriptStructuredChatHandlers = append(scriptStructuredChatHandlers, structuredChatHandler{
		owner: owner, filter: filter, fn: fn, queue: currentScriptEventQueue(owner), registration: registration,
	})
	chatHandlersMu.Unlock()
	return registration
}

func scriptRegisterServerMessage(owner string, filter ServerMessageFilter, fn func(scriptapi.ServerMessage)) scriptRegistrationHandle {
	if scriptIsDisabled(owner) || fn == nil {
		return scriptRegistrationHandle{}
	}
	var registration scriptRegistrationHandle
	registration = registerScriptResource(owner, func() {
		chatHandlersMu.Lock()
		for i := len(scriptServerMessageHandlers) - 1; i >= 0; i-- {
			if scriptServerMessageHandlers[i].registration == registration {
				scriptServerMessageHandlers = append(scriptServerMessageHandlers[:i], scriptServerMessageHandlers[i+1:]...)
			}
		}
		chatHandlersMu.Unlock()
	})
	chatHandlersMu.Lock()
	scriptServerMessageHandlers = append(scriptServerMessageHandlers, serverMessageHandler{
		owner: owner, filter: filter, fn: fn, queue: currentScriptEventQueue(owner), registration: registration,
	})
	chatHandlersMu.Unlock()
	return registration
}

func scriptRegisterLifecycle(owner, kind string, fn func(LifecycleEvent)) scriptRegistrationHandle {
	if scriptIsDisabled(owner) || fn == nil {
		return scriptRegistrationHandle{}
	}
	var registration scriptRegistrationHandle
	registration = registerScriptResource(owner, func() {
		chatHandlersMu.Lock()
		for i := len(scriptLifecycleHandlers) - 1; i >= 0; i-- {
			if scriptLifecycleHandlers[i].registration == registration {
				scriptLifecycleHandlers = append(scriptLifecycleHandlers[:i], scriptLifecycleHandlers[i+1:]...)
			}
		}
		chatHandlersMu.Unlock()
	})
	chatHandlersMu.Lock()
	scriptLifecycleHandlers = append(scriptLifecycleHandlers, scriptLifecycleHandler{
		owner: owner, kind: kind, fn: fn, queue: currentScriptEventQueue(owner), registration: registration,
	})
	chatHandlersMu.Unlock()
	return registration
}

func scriptRegisterChange(owner, kind string, fn func(ChangeEvent)) scriptRegistrationHandle {
	if scriptIsDisabled(owner) || fn == nil {
		return scriptRegistrationHandle{}
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	var registration scriptRegistrationHandle
	registration = registerScriptResource(owner, func() {
		chatHandlersMu.Lock()
		for i := len(scriptChangeHandlers) - 1; i >= 0; i-- {
			if scriptChangeHandlers[i].registration == registration {
				scriptChangeHandlers = append(scriptChangeHandlers[:i], scriptChangeHandlers[i+1:]...)
			}
		}
		chatHandlersMu.Unlock()
	})
	chatHandlersMu.Lock()
	scriptChangeHandlers = append(scriptChangeHandlers, scriptChangeHandler{
		owner: owner, kind: kind, fn: fn, queue: currentScriptEventQueue(owner), registration: registration,
	})
	chatHandlersMu.Unlock()
	return registration
}

func dispatchScriptChange(event ChangeEvent) {
	chatHandlersMu.RLock()
	handlers := append([]scriptChangeHandler{}, scriptChangeHandlers...)
	chatHandlersMu.RUnlock()
	for _, handler := range handlers {
		if handler.fn == nil || handler.kind != "" && handler.kind != event.Type {
			continue
		}
		fn := handler.fn
		queueScriptCallbackOn(handler.queue, handler.owner, "Change "+event.Type, func() { fn(event) })
	}
}

func setScriptLocation(location string) {
	scriptLocationMu.Lock()
	scriptLocation = strings.TrimSpace(location)
	scriptLocationMu.Unlock()
}

func pollScriptChangeEvents() {
	current := captureScriptChangeSnapshot()
	scriptChangeMu.Lock()
	previous := scriptChanges
	scriptChanges = current
	scriptChangeMu.Unlock()
	notifyScriptStateWaiters()
	if !previous.initialized {
		return
	}
	base := ChangeEvent{
		Inventory: append([]InventoryItem(nil), current.inventory...), Equipment: append([]InventoryItem(nil), current.equipment...),
		SelectedPlayer: current.selectedPlayer, SelectedItem: current.selectedItem, HasSelectedItem: current.hasSelectedItem,
		Health: current.health, HealthMax: current.healthMax, Spirit: current.spirit, SpiritMax: current.spiritMax,
		Balance: current.balance, BalanceMax: current.balanceMax, Location: current.location, WorldGeneration: current.worldGeneration,
	}
	if !reflect.DeepEqual(previous.inventory, current.inventory) {
		event := base
		event.Type = ChangeInventory
		dispatchScriptChange(event)
	}
	if !reflect.DeepEqual(previous.equipment, current.equipment) {
		event := base
		event.Type = ChangeEquipment
		dispatchScriptChange(event)
	}
	if previous.selectedPlayer != current.selectedPlayer {
		event := base
		event.Type = ChangeSelectedPlayer
		dispatchScriptChange(event)
	}
	if previous.hasSelectedItem != current.hasSelectedItem || previous.selectedItem != current.selectedItem {
		event := base
		event.Type = ChangeSelectedItem
		dispatchScriptChange(event)
	}
	if previous.health != current.health || previous.healthMax != current.healthMax || previous.spirit != current.spirit ||
		previous.spiritMax != current.spiritMax || previous.balance != current.balance || previous.balanceMax != current.balanceMax {
		event := base
		event.Type = ChangeVitals
		dispatchScriptChange(event)
	}
	if previous.worldGeneration != current.worldGeneration {
		event := base
		event.Type = ChangeWorld
		dispatchScriptChange(event)
	}
	if previous.location != current.location {
		event := base
		event.Type = ChangeLocation
		dispatchScriptChange(event)
	}
}

func captureScriptChangeSnapshot() scriptChangeSnapshot {
	inventory := getInventory()
	equipment := make([]InventoryItem, 0, len(inventory))
	for _, item := range inventory {
		if item.Equipped {
			equipment = append(equipment, item)
		}
	}
	selectedItem, hasSelectedItem := InventoryItem{}, false
	for _, item := range inventory {
		if item.ID == selectedInvID && item.IDIndex == selectedInvIdx {
			selectedItem, hasSelectedItem = item, true
			break
		}
	}
	stateMu.Lock()
	health, healthMax := state.hp, state.hpMax
	spirit, spiritMax := state.sp, state.spMax
	balanceValue, balanceMaxValue := state.balance, state.balanceMax
	stateMu.Unlock()
	scriptLocationMu.RLock()
	location := scriptLocation
	scriptLocationMu.RUnlock()
	return scriptChangeSnapshot{
		initialized: true, inventory: inventory, equipment: equipment, selectedPlayer: selectedPlayerName,
		selectedItem: selectedItem, hasSelectedItem: hasSelectedItem, health: health, healthMax: healthMax,
		spirit: spirit, spiritMax: spiritMax, balance: balanceValue, balanceMax: balanceMaxValue,
		location: location, worldGeneration: worldStateGeneration.Load(),
	}
}

func dispatchScriptLifecycle(event LifecycleEvent) {
	chatHandlersMu.RLock()
	handlers := append([]scriptLifecycleHandler{}, scriptLifecycleHandlers...)
	chatHandlersMu.RUnlock()
	for _, handler := range handlers {
		if handler.kind != event.Type || handler.fn == nil {
			continue
		}
		fn := handler.fn
		queueScriptCallbackOn(handler.queue, handler.owner, "Lifecycle "+event.Type, func() { fn(event) })
	}
}

func scriptSessionLogin(character string) {
	character = strings.TrimSpace(character)
	clearScriptLatestServerMessage()
	scriptSessionMu.Lock()
	previous := scriptSessionCharacter
	changed := previous != "" && !strings.EqualFold(previous, character)
	scriptSessionCharacter = character
	scriptSessionActive = true
	scriptSessionMu.Unlock()
	if changed {
		dispatchScriptLifecycle(LifecycleEvent{Type: lifecycleCharacterChange, Character: character, PreviousCharacter: previous})
	}
	dispatchScriptLifecycle(LifecycleEvent{Type: lifecycleLogin, Character: character, PreviousCharacter: previous})
}

func scriptSessionLogout(character string) {
	scriptSessionMu.Lock()
	if !scriptSessionActive {
		scriptSessionMu.Unlock()
		return
	}
	if character == "" {
		character = scriptSessionCharacter
	}
	scriptSessionActive = false
	scriptSessionMu.Unlock()
	clearScriptLatestServerMessage()
	dispatchScriptLifecycle(LifecycleEvent{Type: lifecycleLogout, Character: character})
}

func runScriptStopHandlers(owner, reason string) {
	chatHandlersMu.RLock()
	handlers := append([]scriptLifecycleHandler{}, scriptLifecycleHandlers...)
	chatHandlersMu.RUnlock()
	event := LifecycleEvent{Type: lifecycleStop, Character: playerName, Reason: reason}
	for _, handler := range handlers {
		if handler.owner == owner && handler.kind == lifecycleStop && handler.fn != nil {
			fn := handler.fn
			queueScriptCallbackWaitOn(handler.queue, owner, "Lifecycle stop", func() { fn(event) })
		}
	}
}

func dispatchScriptChat(msg string) {
	event := classifyScriptChat(msg)
	chatHandlersMu.RLock()
	structuredHandlers := append([]structuredChatHandler{}, scriptStructuredChatHandlers...)
	chatHandlersMu.RUnlock()
	for _, h := range structuredHandlers {
		if h.fn == nil || !scriptChatFilterMatches(h.filter, event) {
			continue
		}
		scriptLogEvent(h.owner, "OnChat", msg)
		fn := h.fn
		queueScriptCallbackOn(h.queue, h.owner, "OnChat", func() { fn(event) })
	}
}

func classifyScriptChat(msg string) ChatEvent {
	speaker := chatSpeaker(msg)
	event := ChatEvent{Speaker: speaker, Message: scriptChatMessage(msg, speaker), Raw: msg, Kinds: ChatAny, Type: scriptChatType(msg)}
	if strings.EqualFold(speaker, playerName) && playerName != "" {
		event.Kinds |= ChatSelf
	} else {
		event.Kinds |= ChatOther
	}
	if speaker == "" {
		event.Kinds |= ChatCreature
		return event
	}
	playersMu.RLock()
	p, known := players[speaker]
	playersMu.RUnlock()
	if known && p != nil && p.IsNPC || isNPCDescriptor(speaker) {
		event.Kinds |= ChatNPC
		return event
	}
	if known || strings.EqualFold(speaker, playerName) && playerName != "" {
		event.Kinds |= ChatPlayer
	} else {
		event.Kinds |= ChatCreature
	}
	return event
}

func scriptChatType(raw string) string {
	message := strings.ToLower(strings.TrimSpace(raw))
	if strings.HasPrefix(message, "(") {
		return ChatTypeEmote
	}
	for _, typedSeparator := range []struct{ separator, eventType string }{
		{" thinks to you", ChatTypeThinkTo},
		{" whispers", ChatTypeWhisper},
		{" exclaims", ChatTypeExclaim},
		{" yells", ChatTypeYell},
		{" asks", ChatTypeAsk},
		{" says", ChatTypeSay},
	} {
		if strings.Contains(message, typedSeparator.separator) {
			return typedSeparator.eventType
		}
	}
	return ""
}

func scriptChatMessage(raw, speaker string) string {
	message := strings.TrimSpace(raw)
	if strings.HasPrefix(message, "(") {
		if end := strings.IndexByte(message, ')'); end >= 0 {
			return strings.TrimSpace(message[end+1:])
		}
	}
	lower := strings.ToLower(message)
	for _, separator := range []string{" says", " yells", " whispers", " asks", " exclaims", " thinks to you"} {
		if index := strings.Index(lower, separator); index > 0 {
			return strings.TrimSpace(strings.TrimLeft(message[index+len(separator):], ",: "))
		}
	}
	if speaker != "" && len(message) > len(speaker) {
		return strings.TrimSpace(message[len(speaker):])
	}
	return message
}

func scriptChatFilterMatches(filter ChatFilter, event ChatEvent) bool {
	if filter.Type != "" && !strings.EqualFold(strings.TrimSpace(filter.Type), event.Type) {
		return false
	}
	if filter.Speaker != "" && !strings.EqualFold(strings.TrimSpace(filter.Speaker), event.Speaker) {
		return false
	}
	if filter.Contains != "" && !strings.Contains(strings.ToLower(event.Message), strings.ToLower(filter.Contains)) {
		return false
	}
	return filter.Kinds == 0 || filter.Kinds&event.Kinds != 0
}

func runServerMessageHandlers(event scriptapi.ServerMessage) {
	scriptLatestServerMessageMu.Lock()
	scriptLatestServerMessageSequence++
	event.Sequence = scriptLatestServerMessageSequence
	event.ReceivedAt = time.Now()
	scriptLatestServerMessageSnapshot = event
	scriptHasLatestServerMessage = true
	scriptLatestServerMessageMu.Unlock()
	chatHandlersMu.RLock()
	handlers := append([]serverMessageHandler{}, scriptServerMessageHandlers...)
	chatHandlersMu.RUnlock()
	for _, handler := range handlers {
		if handler.fn == nil || handler.filter.Type != "" && !strings.EqualFold(handler.filter.Type, event.Type) ||
			handler.filter.Contains != "" && !strings.Contains(strings.ToLower(event.Message), strings.ToLower(handler.filter.Contains)) {
			continue
		}
		fn := handler.fn
		queueScriptCallbackOn(handler.queue, handler.owner, "OnServerMessage", func() { fn(event) })
	}
}

func isNPCDescriptor(name string) bool {
	if name == "" {
		return false
	}
	stateMu.Lock()
	for _, d := range state.descriptors {
		if d.Name == name {
			isNPC := d.Type == kDescNPC
			stateMu.Unlock()
			return isNPC
		}
	}
	stateMu.Unlock()
	return false
}

func scriptPlaySound(ids []uint16) {
	playSound(ids)
}

// ---- Overlay helpers (called by script exports) ----
func scriptOverlayClear(owner string) {
	overlayMu.Lock()
	delete(scriptOverlayOps, owner)
	overlayMu.Unlock()
}

func scriptOverlayRect(owner string, x, y, w, h int, r, g, b, a uint8) {
	if w <= 0 || h <= 0 {
		return
	}
	overlayMu.Lock()
	scriptOverlayOps[owner] = append(scriptOverlayOps[owner], overlayOp{kind: 0, x: x, y: y, w: w, h: h, r: r, g: g, b: b, a: a})
	overlayMu.Unlock()
}

func scriptOverlayText(owner string, x, y int, txt string, r, g, b, a uint8) {
	if txt == "" {
		return
	}
	overlayMu.Lock()
	scriptOverlayOps[owner] = append(scriptOverlayOps[owner], overlayOp{kind: 1, x: x, y: y, text: txt, r: r, g: g, b: b, a: a})
	overlayMu.Unlock()
}

func scriptOverlayImage(owner string, id uint16, x, y int) {
	if id == 0xffff || id == 0 {
		return
	}
	overlayMu.Lock()
	scriptOverlayOps[owner] = append(scriptOverlayOps[owner], overlayOp{kind: 2, x: x, y: y, id: id, a: 255, r: 255, g: 255, b: 255})
	overlayMu.Unlock()
}

type scriptFileState struct {
	size    int64
	modTime int64
	sum     [sha256.Size]byte
}

func snapshotScriptFiles(scriptDirs []string) map[string]scriptFileState {
	snapshot := map[string]scriptFileState{}
	for _, dir := range scriptDirs {
		for _, script := range discoverScriptPackages(dir) {
			snapshot[script.containerPath] = scriptFileState{
				size: script.size, modTime: script.modTime, sum: script.fingerprint,
			}
		}
	}
	return snapshot
}

func isUserScriptFile(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func sameScriptFileSnapshot(a, b map[string]scriptFileState) bool {
	if len(a) != len(b) {
		return false
	}
	for filePath, state := range a {
		if other, ok := b[filePath]; !ok || other != state {
			return false
		}
	}
	return true
}

type scriptInfo struct {
	id          string
	name        string
	author      string
	category    string
	subCategory string
	description string
	path        string
	container   string
	src         []byte
	invalid     bool
	err         string
	apiVer      int
	assets      *scriptAssetSource
	fingerprint [sha256.Size]byte
}

func scanscripts(scriptDirs []string, dup func(name, path string)) map[string]scriptInfo {
	idRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+scriptID\s*=\s*"([^"]+)"`)
	nameRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+scriptName\s*=\s*"([^"]+)"`)
	authorRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+scriptAuthor\s*=\s*"([^"]+)"`)
	categoryRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+scriptCategory\s*=\s*"([^"]+)"`)
	subCategoryRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+scriptSubCategory\s*=\s*"([^"]+)"`)
	descriptionRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+scriptDescription\s*=\s*"([^"]+)"`)
	apiVerRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+scriptAPIVersion\s*=\s*([0-9]+)\s*$`)
	scripts := map[string]scriptInfo{}
	seenNames := map[string]bool{}
	seenIDs := map[string]bool{}
	for _, dir := range scriptDirs {
		for _, script := range discoverScriptPackages(dir) {
			path := script.sourcePath
			if path == "" {
				path = script.containerPath
			}
			src := script.source
			nameMatch := nameRE.FindSubmatch(src)
			base := script.fallbackName
			id := normalizeScriptID(base)
			if match := idRE.FindSubmatch(src); len(match) >= 2 {
				explicitID := strings.TrimSpace(string(match[1]))
				if normalizeScriptID(explicitID) != strings.ToLower(explicitID) {
					consoleMessage("[script] invalid scriptID: " + path)
					id = ""
				} else {
					id = strings.ToLower(explicitID)
				}
			}
			name := base
			if len(nameMatch) >= 2 {
				name = strings.TrimSpace(string(nameMatch[1]))
			}
			catMatch := categoryRE.FindSubmatch(src)
			category := ""
			if len(catMatch) >= 2 {
				category = strings.TrimSpace(string(catMatch[1]))
			}
			subMatch := subCategoryRE.FindSubmatch(src)
			subCategory := ""
			if len(subMatch) >= 2 {
				subCategory = strings.TrimSpace(string(subMatch[1]))
			}
			author := ""
			if match := authorRE.FindSubmatch(src); len(match) >= 2 {
				author = strings.TrimSpace(string(match[1]))
			}
			invalid := script.err != nil
			invalidReason := ""
			if script.err != nil {
				invalidReason = script.err.Error()
			}
			if id == "" {
				invalid = true
				invalidReason = "scriptID must contain only letters, numbers, dashes, and underscores"
			}
			apiVer := scriptAPICurrentVersion
			if len(nameMatch) >= 2 && (name == "" || invalidscriptValue(name)) {
				consoleMessage("[script] invalid name: " + path)
				invalid = true
				invalidReason = "scriptName cannot be empty and must be plain text"
			}
			if author != "" && invalidscriptValue(author) {
				consoleMessage("[script] invalid author: " + path)
				invalid = true
				invalidReason = "scriptAuthor must be plain text"
			}
			if category != "" && invalidscriptValue(category) {
				consoleMessage("[script] invalid category: " + path)
				invalid = true
				invalidReason = "scriptCategory must be plain text"
			}
			if m := apiVerRE.FindSubmatch(src); len(m) >= 2 {
				if n, err := strconv.Atoi(strings.TrimSpace(string(m[1]))); err == nil {
					apiVer = n
				}
			}
			description := ""
			if match := descriptionRE.FindSubmatch(src); len(match) >= 2 {
				description = strings.TrimSpace(string(match[1]))
			}
			lower := strings.ToLower(name)
			if seenNames[lower] {
				if dup != nil {
					dup(name, path)
				}
				continue
			}
			seenNames[lower] = true
			if seenIDs[id] {
				consoleMessage("[script] duplicate scriptID: " + id)
				continue
			}
			seenIDs[id] = true
			owner := id
			scripts[owner] = scriptInfo{
				id:          id,
				name:        name,
				author:      author,
				category:    category,
				subCategory: subCategory,
				description: description,
				path:        path,
				container:   script.containerPath,
				src:         src,
				invalid:     invalid,
				err:         invalidReason,
				apiVer:      apiVer,
				assets:      script.assets,
				fingerprint: script.fingerprint,
			}
		}
	}
	return scripts
}

func normalizeScriptID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var result strings.Builder
	lastDash := false
	for _, char := range value {
		valid := char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '_'
		if valid {
			result.WriteRune(char)
			lastDash = false
			continue
		}
		if char == '-' || char == ' ' || char == '.' {
			if result.Len() > 0 && !lastDash {
				result.WriteByte('-')
				lastDash = true
			}
			continue
		}
		return ""
	}
	return strings.Trim(result.String(), "-")
}

func rescanscripts() {
	if isWASM {
		return
	}
	rescanScripts(scriptSearchDirs())
}

func rescanScripts(scriptDirs []string) {
	scanned := scanscripts(scriptDirs, nil)

	scriptMu.RLock()
	oldOwners := make(map[string]struct{}, len(scriptDisplayNames))
	oldRunning := make(map[string]bool, len(scriptDisplayNames))
	for o := range scriptDisplayNames {
		oldOwners[o] = struct{}{}
		oldRunning[o] = !scriptDisabled[o]
	}
	scriptMu.RUnlock()

	for o := range oldOwners {
		if _, ok := scanned[o]; !ok {
			disablescript(o, "removed")
			scriptMu.Lock()
			delete(scriptActiveSourceHashes, o)
			scriptMu.Unlock()
		}
	}

	reloadOwners := make([]string, 0, len(scanned))
	scriptMu.Lock()
	scriptDisplayNames = make(map[string]string, len(scanned))
	scriptPaths = make(map[string]string, len(scanned))
	scriptAuthors = make(map[string]string, len(scanned))
	scriptCategories = make(map[string]string, len(scanned))
	scriptSubCategories = make(map[string]string, len(scanned))
	scriptDescriptions = make(map[string]string, len(scanned))
	scriptPackages = make(map[string]scriptInfo, len(scanned))
	scriptAPIVersions = make(map[string]int, len(scanned))
	scriptInvalid = make(map[string]bool, len(scanned))
	scriptDisabled = make(map[string]bool, len(scanned))
	newErrors := make(map[string]string, len(scanned))
	newReloadFailed := make(map[string]bool, len(scanned))
	newEnabled := map[string]scriptScope{}
	for o, info := range scanned {
		scriptPackages[o] = info
		scriptDisplayNames[o] = info.name
		scriptPaths[o] = info.path
		scriptAuthors[o] = info.author
		scriptCategories[o] = info.category
		scriptSubCategories[o] = info.subCategory
		scriptDescriptions[o] = info.description
		scriptAPIVersions[o] = info.apiVer
		if message := scriptErrors[o]; message != "" {
			newErrors[o] = message
			newReloadFailed[o] = scriptReloadFailed[o]
		}
		if en, ok := scriptEnabledFor[o]; ok {
			newEnabled[o] = en
		} else if gs.Enabledscripts != nil {
			if val, ok := gs.Enabledscripts[o]; ok {
				newEnabled[o] = scopeFromSettingValue(val)
			}
		}
		// Require a matching script API version
		invalid := info.invalid || info.apiVer != scriptAPICurrentVersion
		if info.err != "" {
			newErrors[o] = formatScriptError(info.path, fmt.Errorf("%s", info.err))
		}
		if info.apiVer != scriptAPICurrentVersion {
			newErrors[o] = fmt.Sprintf("%s: unsupported script API version %d; this client supports version %d", info.path, info.apiVer, scriptAPICurrentVersion)
		}
		scriptInvalid[o] = invalid
		if invalid {
			scriptDisabled[o] = !oldRunning[o]
			continue
		}
		effChar := playerName
		if effChar == "" {
			effChar = gs.LastCharacter
		}
		shouldEnable := newEnabled[o].enablesFor(effChar)
		scriptDisabled[o] = !oldRunning[o]
		activeHash, hasActiveHash := scriptActiveSourceHashes[o]
		if oldRunning[o] && shouldEnable && (!hasActiveHash || activeHash != info.fingerprint) {
			reloadOwners = append(reloadOwners, o)
		}
	}
	scriptEnabledFor = newEnabled
	scriptErrors = newErrors
	scriptReloadFailed = newReloadFailed
	scriptNames = make(map[string]bool, len(scanned))
	for _, info := range scanned {
		scriptNames[strings.ToLower(info.name)] = true
	}
	scriptMu.Unlock()

	sort.Strings(reloadOwners)
	for _, owner := range reloadOwners {
		info := scanned[owner]
		loadscriptPackageSource(owner, info.name, info.path, info.src, restrictedStdlib(), info.assets, info.fingerprint)
	}
	applyEnabledScripts()
	refreshscriptsWindow()
	settingsDirty = true
}

func checkForScriptEdit() {
	if isWASM {
		return
	}
	if time.Since(scriptModCheck) < 500*time.Millisecond {
		return
	}
	scriptModCheck = time.Now()
	current := snapshotScriptFiles(scriptSearchDirs())
	if !sameScriptFileSnapshot(scriptFileSnapshot, current) {
		scriptFileSnapshot = current
		rescanscripts()
	}
}

func loadScripts() {
	if isWASM {
		return
	}
	ensureScriptsDir()
	scanned := scanscripts(scriptSearchDirs(), func(name, path string) {
		log.Printf("script %s duplicate name %s", path, name)
		consoleMessage("[script] duplicate name: " + name)
	})

	scriptNames = make(map[string]bool, len(scanned))
	scriptMu.Lock()
	scriptPackages = make(map[string]scriptInfo, len(scanned))
	for owner, info := range scanned {
		scriptPackages[owner] = info
	}
	scriptMu.Unlock()
	owners := make([]string, 0, len(scanned))
	for owner := range scanned {
		owners = append(owners, owner)
	}
	sort.Strings(owners)
	for _, o := range owners {
		info := scanned[o]
		scriptNames[strings.ToLower(info.name)] = true
		s, ok := scriptEnabledFor[o]
		if !ok && gs.Enabledscripts != nil {
			if val, ok2 := gs.Enabledscripts[o]; ok2 {
				s = scopeFromSettingValue(val)
			}
		}
		effChar := playerName
		if effChar == "" {
			effChar = gs.LastCharacter
		}
		invalid := info.invalid || info.apiVer != scriptAPICurrentVersion
		disabled := invalid || !s.enablesFor(effChar)
		scriptMu.Lock()
		scriptDisplayNames[o] = info.name
		scriptCategories[o] = info.category
		scriptSubCategories[o] = info.subCategory
		scriptDescriptions[o] = info.description
		scriptAPIVersions[o] = info.apiVer
		scriptPaths[o] = info.path
		if !s.empty() {
			scriptEnabledFor[o] = s
		}
		scriptAuthors[o] = info.author
		scriptInvalid[o] = invalid
		scriptDisabled[o] = disabled
		if info.err != "" {
			scriptErrors[o] = formatScriptError(info.path, fmt.Errorf("%s", info.err))
		}
		if info.apiVer != scriptAPICurrentVersion {
			scriptErrors[o] = fmt.Sprintf("%s: unsupported script API version %d; this client supports version %d", info.path, info.apiVer, scriptAPICurrentVersion)
		}
		scriptMu.Unlock()
		if !disabled {
			loadscriptPackageSource(o, info.name, info.path, info.src, restrictedStdlib(), info.assets, info.fingerprint)
		}
	}
	refreshHotkeysList()
	refreshscriptsWindow()
	scriptFileSnapshot = snapshotScriptFiles(scriptSearchDirs())
}

// scopeFromSettingValue converts a settings value into a scriptScope.
// Accepted values:
// - string("all"): All=true
// - string(name): include that character
// - []string: include all listed characters
// - []any (from JSON): include all listed string characters
// - bool(true): include LastCharacter if present
func scopeFromSettingValue(v any) scriptScope {
	s := scriptScope{}
	switch val := v.(type) {
	case string:
		if val == "all" {
			s.All = true
		} else if val != "" {
			s.addChar(val)
		}
	case []string:
		for _, n := range val {
			if n != "" {
				s.addChar(n)
			}
		}
	case []any:
		for _, e := range val {
			if str, ok := e.(string); ok && str != "" {
				s.addChar(str)
			}
		}
	case bool:
		if val && gs.LastCharacter != "" {
			s.addChar(gs.LastCharacter)
		}
	}
	return s
}
