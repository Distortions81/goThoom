package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

const scriptAPICurrentVersion = 1

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

// Expose the script API under both a short and a module-qualified path so
// Yaegi can resolve imports regardless of how the script refers to it.
var basescriptExports = interp.Exports{
	// Short path used by simple script scripts: import "gt"
	// Yaegi expects keys as "importPath/pkgName".
	"gt/gt": {
		"ShowNotification":    reflect.ValueOf(scriptShowNotification),
		"CLVersion":           reflect.ValueOf(&clVersion).Elem(),
		"PlayerName":          reflect.ValueOf(scriptPlayerName),
		"Players":             reflect.ValueOf(scriptPlayers),
		"Inventory":           reflect.ValueOf(scriptInventory),
		"InventoryItem":       reflect.ValueOf((*InventoryItem)(nil)),
		"Player":              reflect.ValueOf((*Player)(nil)),
		"PlaySound":           reflect.ValueOf(scriptPlaySound),
		"InputText":           reflect.ValueOf(scriptInputText),
		"SetInputText":        reflect.ValueOf(scriptSetInputText),
		"KeyJustPressed":      reflect.ValueOf(scriptKeyJustPressed),
		"MouseJustPressed":    reflect.ValueOf(scriptMouseJustPressed),
		"MouseWheel":          reflect.ValueOf(scriptMouseWheel),
		"LastClick":           reflect.ValueOf(scriptLastClick),
		"ClickInfo":           reflect.ValueOf((*ClickInfo)(nil)),
		"InputEvent":          reflect.ValueOf((*InputEvent)(nil)),
		"ChatFilter":          reflect.ValueOf((*ChatFilter)(nil)),
		"ChatEvent":           reflect.ValueOf((*ChatEvent)(nil)),
		"ServerMessageFilter": reflect.ValueOf((*ServerMessageFilter)(nil)),
		"ServerMessageEvent":  reflect.ValueOf((*ServerMessageEvent)(nil)),
		"LifecycleEvent":      reflect.ValueOf((*LifecycleEvent)(nil)),
		"ChangeEvent":         reflect.ValueOf((*ChangeEvent)(nil)),
		"Subscription":        reflect.ValueOf((*Subscription)(nil)),
		"Timer":               reflect.ValueOf((*Timer)(nil)),
		"Mobile":              reflect.ValueOf((*Mobile)(nil)),
		"EquippedItems":       reflect.ValueOf(scriptEquippedItems),
		"HasItem":             reflect.ValueOf(scriptHasItem),
		"IsEquipped":          reflect.ValueOf(scriptIsEquipped),
		"IgnoreCase":          reflect.ValueOf(scriptIgnoreCase),
		"StartsWith":          reflect.ValueOf(scriptStartsWith),
		"EndsWith":            reflect.ValueOf(scriptEndsWith),
		"Includes":            reflect.ValueOf(scriptIncludes),
		"Lower":               reflect.ValueOf(scriptLower),
		"Upper":               reflect.ValueOf(scriptUpper),
		"Trim":                reflect.ValueOf(scriptTrim),
		"TrimStart":           reflect.ValueOf(scriptTrimStart),
		"TrimEnd":             reflect.ValueOf(scriptTrimEnd),
		"Words":               reflect.ValueOf(scriptWords),
		"Join":                reflect.ValueOf(scriptJoin),
		"Replace":             reflect.ValueOf(scriptReplace),
		"Split":               reflect.ValueOf(scriptSplit),
		// Chat trigger flags
		"ChatAny":              reflect.ValueOf(ChatAny),
		"ChatPlayer":           reflect.ValueOf(ChatPlayer),
		"ChatNPC":              reflect.ValueOf(ChatNPC),
		"ChatCreature":         reflect.ValueOf(ChatCreature),
		"ChatSelf":             reflect.ValueOf(ChatSelf),
		"ChatOther":            reflect.ValueOf(ChatOther),
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
		// Prefer string-based APIs; keep ID variants for power users
		m["Equip"] = reflect.ValueOf(func(name string) { stage(func() { scriptEquipByName(owner, name) }) })
		m["Unequip"] = reflect.ValueOf(func(name string) { stage(func() { scriptUnequipByName(owner, name) }) })
		m["EquipPartial"] = reflect.ValueOf(func(name string) { stage(func() { scriptEquipPartial(owner, name) }) })
		m["UnequipPartial"] = reflect.ValueOf(func(name string) { stage(func() { scriptUnequipPartial(owner, name) }) })
		m["EquipById"] = reflect.ValueOf(func(id uint16) { stage(func() { scriptEquip(owner, id) }) })
		m["UnequipById"] = reflect.ValueOf(func(id uint16) { stage(func() { scriptUnequip(owner, id) }) })
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
		m["AddHotkey"] = reflect.ValueOf(func(combo, command string) {
			if candidate.claimBinding(combo, command) {
				stage(func() { scriptAddHotkey(owner, combo, command) })
			}
		})
		m["Bind"] = reflect.ValueOf(func(combo string, handler func(InputEvent)) Subscription {
			if candidate.claimBinding(combo, handler) {
				return subscribe(func() scriptRegistrationHandle { return scriptAddHotkeyFn(owner, combo, handler) })
			}
			return Subscription{}
		})
		m["RemoveHotkey"] = reflect.ValueOf(func(combo string) { stage(func() { scriptRemoveHotkey(owner, combo) }) })
		m["RegisterCommand"] = reflect.ValueOf(func(name string, handler scriptCommandHandler) {
			if candidate.claimCommand(name, handler) {
				stage(func() { scriptRegisterCommand(owner, name, handler) })
			}
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
		m["AddShortcuts"] = reflect.ValueOf(func(shortcuts map[string]string) {
			copyOfShortcuts := make(map[string]string, len(shortcuts))
			for short, full := range shortcuts {
				copyOfShortcuts[short] = full
			}
			stage(func() { scriptAddShortcuts(owner, copyOfShortcuts) })
		})
		// Chat/Console (simple, no slices)
		// Simple DSL aliases
		m["Print"] = reflect.ValueOf(func(msg string) { stage(func() { scriptConsole(msg) }) })
		m["ShowNotification"] = reflect.ValueOf(func(msg string) { stage(func() { scriptShowNotification(msg) }) })
		m["Notify"] = reflect.ValueOf(func(msg string) { stage(func() { scriptShowNotification(msg) }) })
		m["PlaySound"] = reflect.ValueOf(func(ids []uint16) {
			copyOfIDs := append([]uint16(nil), ids...)
			stage(func() { scriptPlaySound(copyOfIDs) })
		})
		m["Cmd"] = reflect.ValueOf(func(text string) {
			text = strings.TrimSpace(text)
			stage(func() { scriptCommand(owner, text) })
		})
		m["Send"] = reflect.ValueOf(func(text string) {
			text = strings.TrimSpace(text)
			stage(func() { scriptCommand(owner, text) })
		})
		m["Run"] = reflect.ValueOf(func(text string) {
			text = strings.TrimSpace(text)
			stage(func() { scriptCommand(owner, text) })
		})
		m["Me"] = reflect.ValueOf(scriptPlayerName)
		m["Has"] = reflect.ValueOf(func(name string) bool { return scriptHasItem(name) })
		m["Save"] = reflect.ValueOf(func(key, value string) { candidate.setStorage(owner, key, value) })
		m["Load"] = reflect.ValueOf(func(key string) string {
			if v, ok := candidate.getStorage(owner, key).(string); ok {
				return v
			}
			return ""
		})
		m["Delete"] = reflect.ValueOf(func(key string) { candidate.deleteStorage(owner, key) })
		m["Input"] = reflect.ValueOf(scriptInputText)
		m["SetInput"] = reflect.ValueOf(func(text string) { stage(func() { scriptSetInputText(text) }) })
		m["SetInputText"] = reflect.ValueOf(func(text string) { stage(func() { scriptSetInputText(text) }) })
		// (Removed explicit Thank/Curse/Share/Unshare helpers to avoid duplicating
		// in-game commands; authors can use Cmd("/thank ...") etc.)
		// No-slice chat/console helpers (one call per phrase)
		m["Chat"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				stage(func() { scriptRegisterChat(owner, "", []string{p}, ChatAny, handler) })
			}
		})
		m["OnChat"] = reflect.ValueOf(func(filter ChatFilter, handler func(ChatEvent)) Subscription {
			return subscribe(func() scriptRegistrationHandle { return scriptRegisterStructuredChat(owner, filter, handler) })
		})
		m["OnServerMessage"] = reflect.ValueOf(func(filter ServerMessageFilter, handler func(ServerMessageEvent)) Subscription {
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
		m["PlayerChat"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				stage(func() { scriptRegisterChat(owner, "", []string{p}, ChatPlayer, handler) })
			}
		})
		m["NPCChat"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				stage(func() { scriptRegisterChat(owner, "", []string{p}, ChatNPC, handler) })
			}
		})
		m["CreatureChat"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				stage(func() { scriptRegisterChat(owner, "", []string{p}, ChatCreature, handler) })
			}
		})
		m["SelfChat"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				stage(func() { scriptRegisterChat(owner, "", []string{p}, ChatSelf, handler) })
			}
		})
		m["OtherChat"] = reflect.ValueOf(func(name, phrase string, handler func(string)) {
			n := strings.TrimSpace(name)
			p := strings.TrimSpace(phrase)
			if p != "" {
				stage(func() { scriptRegisterChat(owner, n, []string{p}, ChatOther, handler) })
			}
		})
		m["ChatFrom"] = reflect.ValueOf(func(name, phrase string, handler func(string)) {
			n := strings.TrimSpace(name)
			p := strings.TrimSpace(phrase)
			if n != "" && p != "" {
				stage(func() { scriptRegisterChat(owner, n, []string{p}, ChatAny, handler) })
			}
		})
		m["PlayerChatFrom"] = reflect.ValueOf(func(name, phrase string, handler func(string)) {
			n := strings.TrimSpace(name)
			p := strings.TrimSpace(phrase)
			if n != "" && p != "" {
				stage(func() { scriptRegisterChat(owner, n, []string{p}, ChatPlayer, handler) })
			}
		})
		m["OtherChatFrom"] = reflect.ValueOf(func(name, phrase string, handler func(string)) {
			n := strings.TrimSpace(name)
			p := strings.TrimSpace(phrase)
			if n != "" && p != "" {
				stage(func() { scriptRegisterChat(owner, n, []string{p}, ChatOther, handler) })
			}
		})
		m["ConsoleMsg"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				stage(func() { scriptRegisterConsole(owner, []string{p}, handler) })
			}
		})
		// Sleep for game ticks (blocks current goroutine only)
		m["SleepTicks"] = reflect.ValueOf(func(ticks int) {
			if eventQueue := candidate.runtimeEventQueue(owner); eventQueue != nil {
				scriptSleepTicks(owner, eventQueue, ticks)
			}
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
		// Simpler alias: Console("text", fn)
		m["Console"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				stage(func() { scriptRegisterConsole(owner, []string{p}, handler) })
			}
		})
		// Back-compat registrations matching gt stubs
		m["RegisterTriggers"] = reflect.ValueOf(func(name string, phrases []string, fn func(string)) {
			if fn == nil || len(phrases) == 0 {
				return
			}
			copyOfPhrases := append([]string(nil), phrases...)
			stage(func() { scriptRegisterChat(owner, name, copyOfPhrases, ChatAny, fn) })
		})
		m["RegisterConsoleTriggers"] = reflect.ValueOf(func(phrases []string, fn func()) {
			if fn == nil || len(phrases) == 0 {
				return
			}
			copyOfPhrases := append([]string(nil), phrases...)
			stage(func() { scriptRegisterConsoleTriggers(owner, copyOfPhrases, fn) })
		})
		m["RegisterTrigger"] = reflect.ValueOf(func(name, phrase string, fn func()) {
			p := strings.TrimSpace(phrase)
			if p == "" || fn == nil {
				return
			}
			stage(func() { scriptRegisterChat(owner, name, []string{p}, ChatAny, func(string) { fn() }) })
		})
		m["RegisterPlayerHandler"] = reflect.ValueOf(func(fn func(Player)) {
			stage(func() { scriptRegisterPlayerHandler(owner, fn) })
		})
		m["RegisterInputHandler"] = reflect.ValueOf(func(fn func(string) string) {
			stage(func() { scriptRegisterInputHandler(owner, fn) })
		})
		m["RegisterChatHandler"] = reflect.ValueOf(func(fn func(string)) {
			stage(func() { scriptRegisterChatHandler(owner, fn) })
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
		m["RunCommand"] = reflect.ValueOf(func(cmd string) { stage(func() { scriptCommand(owner, cmd) }) })
		m["EnqueueCommand"] = reflect.ValueOf(func(cmd string) { stage(func() { scriptCommand(owner, cmd) }) })
		m["StorageGet"] = reflect.ValueOf(func(key string) any { return candidate.getStorage(owner, key) })
		m["StorageSet"] = reflect.ValueOf(func(key string, value any) { candidate.setStorage(owner, key, value) })
		m["StorageDelete"] = reflect.ValueOf(func(key string) { candidate.deleteStorage(owner, key) })
		m["AddConfig"] = reflect.ValueOf(func(name, typ string, args ...any) any {
			entry, ok := makeScriptConfigEntry(owner, name, typ, args...)
			if !ok {
				return nil
			}
			stage(func() { scriptRegisterConfig(owner, entry) })
			return entry.Value
		})

		// Timers
		m["After"] = reflect.ValueOf(func(ms int, fn func()) {
			if fn == nil || ms <= 0 {
				return
			}
			stage(func() {
				eventQueue := currentScriptEventQueue(owner)
				t := time.AfterFunc(time.Duration(ms)*time.Millisecond, func() {
					queueScriptCallbackOn(eventQueue, owner, "After", fn)
				})
				scriptMu.Lock()
				scriptTimers[owner] = append(scriptTimers[owner], t)
				scriptMu.Unlock()
			})
		})
		m["AfterDur"] = reflect.ValueOf(func(d time.Duration, fn func()) {
			if fn == nil || d <= 0 {
				return
			}
			stage(func() {
				eventQueue := currentScriptEventQueue(owner)
				t := time.AfterFunc(d, func() { queueScriptCallbackOn(eventQueue, owner, "AfterDur", fn) })
				scriptMu.Lock()
				scriptTimers[owner] = append(scriptTimers[owner], t)
				scriptMu.Unlock()
			})
		})
		m["Every"] = reflect.ValueOf(func(ms int, fn func()) {
			if fn == nil || ms <= 0 {
				return
			}
			stage(func() {
				startScriptRepeat(owner, currentScriptEventQueue(owner), time.Duration(ms)*time.Millisecond, "Every", fn)
			})
		})
		m["EveryDur"] = reflect.ValueOf(func(d time.Duration, fn func()) {
			if fn == nil || d <= 0 {
				return
			}
			stage(func() {
				startScriptRepeat(owner, currentScriptEventQueue(owner), d, "EveryDur", fn)
			})
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

		// Key binding to a function. Keep this simple form alongside
		// AddHotkeyFn for scripts that do not need the event details.
		m["Key"] = reflect.ValueOf(func(combo string, handler func()) {
			if candidate.claimBinding(combo, handler) {
				stage(func() { scriptAddKey(owner, combo, handler) })
			}
		})
		ex[pkg] = m
	}
	return ex
}

//go:embed scripts
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

// ensureScriptsDir creates the scripts directory next to the executable and
// populates it with the embedded scripts if it is missing.
func ensureScriptsDir() {
	if isWASM {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Join(filepath.Dir(exe), "scripts")
	if _, err := os.Stat(dir); err == nil {
		return
	} else if !os.IsNotExist(err) {
		log.Printf("check scripts dir: %v", err)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("create scripts dir: %v", err)
		return
	}
	entries, err := scriptScripts.ReadDir("scripts")
	if err != nil {
		log.Printf("read embedded scripts: %v", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		data, err := scriptScripts.ReadFile(path.Join("scripts", e.Name()))
		if err != nil {
			log.Printf("read embedded %s: %v", e.Name(), err)
			continue
		}
		dst := filepath.Join(dir, e.Name())
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			log.Printf("write %s: %v", dst, err)
		}
	}
}

// ensureDefaultScripts creates the user scripts directory and populates it
// with example scripts when it is empty.
func ensureDefaultScripts() {
	if isWASM {
		return
	}
	dir := userScriptsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("create scripts dir: %v", err)
		return
	}
	// Check if directory already has any .go script files
	hasGo := false
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				hasGo = true
				break
			}
		}
	}
	if hasGo {
		return
	}
	// Write example script files
	files := []string{
		"default_shortcuts.go",
		"README.txt",
		"numpad_poser.go",
	}
	for _, src := range files {
		sPath := path.Join("scripts", src)
		data, err := scriptScripts.ReadFile(sPath)
		if err != nil {
			log.Printf("read embedded %s: %v", sPath, err)
			continue
		}
		base := filepath.Base(sPath)
		dst := filepath.Join(dir, base)
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			log.Printf("write %s: %v", dst, err)
			continue
		}
	}
}

// ensurescriptAPI removed: the editor stub now ships in scripts/gt.

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

// scriptAddKey registers a simple function hotkey without routing it through
// the server-command queue.
func scriptAddKey(owner, combo string, handler func()) {
	c := strings.TrimSpace(combo)
	if c == "" || handler == nil {
		return
	}
	scriptAddHotkeyFn(owner, c, func(InputEvent) { handler() })
}

func scriptAddHotkey(owner, combo, command string) {
	if scriptIsDisabled(owner) {
		return
	}
	combo = strings.TrimSpace(combo)
	if combo == "" {
		return
	}
	if bindingOwner, conflict := scriptBindingConflict(owner, combo); conflict {
		reportScriptBindingConflict(combo, bindingOwner)
		return
	}
	// Default script hotkeys to enabled on first add; users can disable them
	// in the Hotkeys window. Persisted preferences still override this.
	hk := Hotkey{Name: command, Combo: combo, Commands: []HotkeyCommand{{Command: command}}, Script: owner, Disabled: false}
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
			return
		}
	}
	var registration scriptRegistrationHandle
	registration = registerScriptResource(owner, func() {
		removeScriptHotkeyByHandle(registration)
	})
	hk.registration = registration
	hotkeys = append(hotkeys, hk)
	hotkeysMu.Unlock()
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
	name := scriptDisplayNames[owner]
	if name == "" {
		name = owner
	}
	msg := fmt.Sprintf("[script:%s] hotkey added: %s -> %s", name, combo, command)
	if gs.scriptOutputDebug {
		consoleMessage(msg)
	}
	log.Print(msg)
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
	name := scriptDisplayNames[owner]
	if name == "" {
		name = owner
	}
	msg := fmt.Sprintf("[script:%s] hotkey added: %s -> <function>", name, combo)
	if gs.scriptOutputDebug {
		consoleMessage(msg)
	}
	log.Print(msg)
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

type triggerHandler struct {
	owner        string
	name         string
	flags        int
	fn           func(string)
	queue        *scriptEventQueue
	registration scriptRegistrationHandle
}

type inputHandler struct {
	owner        string
	fn           func(string) string
	queue        *scriptEventQueue
	registration scriptRegistrationHandle
}

// chatHandler holds a script-owned handler for all chat messages.
type chatHandler struct {
	owner        string
	fn           func(string)
	queue        *scriptEventQueue
	registration scriptRegistrationHandle
}

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
	fn           func(ServerMessageEvent)
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
	scriptCommands        = map[string]scriptCommandHandler{}
	scriptMu              sync.RWMutex
	scriptNames           = map[string]bool{}
	scriptDisplayNames    = map[string]string{}
	scriptAuthors         = map[string]string{}
	scriptCategories      = map[string]string{}
	scriptSubCategories   = map[string]string{}
	scriptInvalid         = map[string]bool{}
	scriptDisabled        = map[string]bool{}
	scriptEnabledFor      = map[string]scriptScope{}
	scriptPaths           = map[string]string{}
	scriptTerminators     = map[string]func(){}
	scriptTriggers        = map[string][]triggerHandler{}
	scriptConsoleTriggers = map[string][]triggerHandler{}
	triggerHandlersMu     sync.RWMutex
	// Handlers that receive every chat message (no phrase filtering)
	scriptChatHandlers           []chatHandler
	scriptStructuredChatHandlers []structuredChatHandler
	scriptServerMessageHandlers  []serverMessageHandler
	scriptLifecycleHandlers      []scriptLifecycleHandler
	scriptChangeHandlers         []scriptChangeHandler
	chatHandlersMu               sync.RWMutex
	scriptInputHandlers          []inputHandler
	inputHandlersMu              sync.RWMutex
	scriptCommandOwners          = map[string]string{}
	scriptSendHistory            = map[string][]time.Time{}
	scriptFileSnapshot           map[string]scriptFileState
	scriptModCheck               time.Time
	// timers per script owner
	scriptTimers      = map[string][]*time.Timer{}
	scriptTickerStops = map[string][]chan struct{}{}
	scriptTickWaiters = map[string][]*tickWaiter{}
	scriptStopping    = map[string]bool{}

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
	stop := make(chan struct{})
	scriptMu.Lock()
	if scriptDisabled[owner] {
		scriptMu.Unlock()
		return nil
	}
	scriptTickerStops[owner] = append(scriptTickerStops[owner], stop)
	scriptMu.Unlock()
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				queueScriptCallbackOn(eventQueue, owner, event, fn)
			case <-stop:
				return
			}
		}
	}()
	return func() { stopScriptRepeat(owner, stop) }
}

func stopScriptRepeat(owner string, stop chan struct{}) {
	if stop == nil {
		return
	}
	scriptMu.Lock()
	list := scriptTickerStops[owner]
	for i, candidate := range list {
		if candidate != stop {
			continue
		}
		close(stop)
		list = append(list[:i], list[i+1:]...)
		if len(list) == 0 {
			delete(scriptTickerStops, owner)
		} else {
			scriptTickerStops[owner] = list
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
	consoleMessage("[script] command registered: /" + key)
	log.Printf("[script] command registered: /%s", key)
	return registration
}

// scriptCommand is the single path for script-generated server commands. All
// public command aliases use the same FIFO queue and per-script throttle.
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

// These wrappers remain for internal compatibility. They intentionally have
// identical ordering and throttling semantics.
func scriptRunCommand(owner, cmd string) {
	scriptCommand(owner, cmd)
}

func scriptEnqueueCommand(owner, cmd string) {
	scriptCommand(owner, cmd)
}

type preparedScript struct {
	candidate   *scriptCandidate
	initialize  func()
	terminate   func()
	interpreter *interp.Interpreter
	diagnostics *scriptDiagnostics
}

func compileScriptSource(owner string, src []byte, restricted interp.Exports) (*preparedScript, error) {
	candidate := &scriptCandidate{}
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
	if v, err := i.Eval("Init"); err == nil {
		if fn, ok := v.Interface().(func()); ok {
			prepared.initialize = fn
		}
	}
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
	prepared, err := compileScriptSource(owner, src, restricted)
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

func handleScriptCallbackPanic(owner, event string, recovered any, location string) {
	name := scriptDisplayName(owner)
	where := ""
	if location != "" {
		where = " at " + location
	}
	msg := fmt.Sprintf("[script:%s] %s callback panic%s: %v", name, event, where, recovered)
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
	wasRunning := scriptIsRunning(owner)
	prepared, err := prepareScriptSource(owner, src, restricted)
	if err == nil {
		err = scriptCandidateConflict(owner, prepared.candidate)
		if err != nil {
			disposePreparedScript(prepared)
		}
	}
	if err != nil {
		log.Printf("script %s: %v", path, err)
		if wasRunning {
			consoleMessage("[script] reload error for " + path + ": " + err.Error())
			refreshscriptsWindow()
		} else {
			consoleMessage("[script] load error for " + path + ": " + err.Error())
			disablescript(owner, "load error")
		}
		return false
	}
	if wasRunning {
		disablescript(owner, "reloaded")
	}
	activatePreparedScript(owner, prepared)
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
	path := scriptPaths[owner]
	name := scriptDisplayNames[owner]
	scriptMu.RUnlock()
	if path == "" {
		return
	}
	src, err := os.ReadFile(path)
	if err != nil {
		log.Printf("read script %s: %v", path, err)
		consoleMessage("[script] read error for " + path + ": " + err.Error())
		return
	}
	loadscriptSource(owner, name, path, src, restrictedStdlib())
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
	refreshTriggersList()
	// Clear overlay ops
	overlayMu.Lock()
	delete(scriptOverlayOps, owner)
	overlayMu.Unlock()
	// Stop any timers/tickers and tick waiters for this script
	scriptMu.Lock()
	if list := scriptTimers[owner]; len(list) > 0 {
		for _, t := range list {
			if t != nil {
				t.Stop()
			}
		}
		delete(scriptTimers, owner)
	}
	if stops := scriptTickerStops[owner]; len(stops) > 0 {
		for _, ch := range stops {
			if ch != nil {
				close(ch)
			}
		}
		delete(scriptTickerStops, owner)
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
	scriptMu.Unlock()
	// Stop scheduled timers/tickers for this script
	if list := scriptTimers[owner]; len(list) > 0 {
		for _, t := range list {
			if t != nil {
				t.Stop()
			}
		}
		delete(scriptTimers, owner)
	}
	if stops := scriptTickerStops[owner]; len(stops) > 0 {
		for _, ch := range stops {
			if ch != nil {
				close(ch)
			}
		}
		delete(scriptTickerStops, owner)
	}
	applyEnabledScripts()
	saveSettings()
	refreshscriptsWindow()
}

func scriptPlayerName() string {
	return playerName
}

func scriptPlayers() []Player {
	ps := getPlayers()
	out := make([]Player, len(ps))
	copy(out, ps)
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

func scriptEquip(owner string, id uint16) {
	items := getInventory()
	idx := -1
	found := false
	for _, it := range items {
		if it.ID != id {
			continue
		}
		found = true
		if it.Equipped {
			name := it.Name
			if name == "" {
				name = fmt.Sprintf("%d", id)
			}
			consoleMessage(name + " already equipped, skipping")
			return
		}
		if idx < 0 {
			idx = it.IDIndex
		}
	}
	if !found {
		reportScriptCommandError(owner, fmt.Sprintf("equip target not found: id %d", id))
		return
	}
	if scriptCommand(owner, formatEquipCommand(id, idx)) {
		equipInventoryItem(id, idx, true)
	}
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

func scriptUnequip(owner string, id uint16) {
	items := getInventory()
	equipped := false
	for _, it := range items {
		if it.ID == id && it.Equipped {
			equipped = true
			break
		}
	}
	if !equipped {
		reportScriptCommandError(owner, fmt.Sprintf("unequip target not equipped: id %d", id))
		return
	}
	if scriptCommand(owner, fmt.Sprintf("/unequip %d", id)) {
		equipInventoryItem(id, -1, false)
	}
}

// scriptEquipPartial equips the first item whose name contains the pattern
// (case-insensitive). If a matching item is already equipped, it skips.
func scriptEquipPartial(owner, pattern string) {
	p := strings.ToLower(strings.TrimSpace(pattern))
	if p == "" {
		reportScriptCommandError(owner, "equip target not found: empty name")
		return
	}
	items := getInventory()
	var id uint16
	idx := -1
	found := false
	// If any matching item is already equipped, skip as redundant.
	for _, it := range items {
		if strings.Contains(strings.ToLower(it.Name), p) && it.Equipped {
			consoleMessage(it.Name + " already equipped, skipping")
			return
		}
	}
	for _, it := range items {
		if strings.Contains(strings.ToLower(it.Name), p) {
			id = it.ID
			idx = it.IDIndex
			found = true
			break
		}
	}
	if !found {
		reportScriptCommandError(owner, "equip target not found: "+pattern)
		return
	}
	if scriptCommand(owner, formatEquipCommand(id, idx)) {
		equipInventoryItem(id, idx, true)
	}
}

// scriptUnequipPartial unequips any equipped item whose name contains the
// provided pattern (case-insensitive).
func scriptUnequipPartial(owner, pattern string) {
	p := strings.ToLower(strings.TrimSpace(pattern))
	if p == "" {
		reportScriptCommandError(owner, "unequip target not found: empty name")
		return
	}
	items := getInventory()
	for _, it := range items {
		if it.Equipped && strings.Contains(strings.ToLower(it.Name), p) {
			if scriptCommand(owner, fmt.Sprintf("/unequip %d", it.ID)) {
				equipInventoryItem(it.ID, -1, false)
			}
			return
		}
	}
	reportScriptCommandError(owner, "unequip target not equipped: "+pattern)
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

func scriptRegisterInputHandler(owner string, fn func(string) string) scriptRegistrationHandle {
	if scriptIsDisabled(owner) || fn == nil {
		return scriptRegistrationHandle{}
	}
	var registration scriptRegistrationHandle
	registration = registerScriptResource(owner, func() {
		removeScriptInputHandlerByHandle(registration)
	})
	inputHandlersMu.Lock()
	scriptInputHandlers = append(scriptInputHandlers, inputHandler{owner: owner, fn: fn, queue: currentScriptEventQueue(owner), registration: registration})
	inputHandlersMu.Unlock()
	return registration
}

func removeScriptInputHandlerByHandle(registration scriptRegistrationHandle) {
	inputHandlersMu.Lock()
	for i := len(scriptInputHandlers) - 1; i >= 0; i-- {
		if scriptInputHandlers[i].registration == registration {
			scriptInputHandlers = append(scriptInputHandlers[:i], scriptInputHandlers[i+1:]...)
		}
	}
	inputHandlersMu.Unlock()
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

// ChatFilter selects structured chat events. Zero values match all chat.
type ChatFilter struct {
	Contains string
	Speaker  string
	Kinds    int
}

// ChatEvent contains parsed chat data while retaining the original line.
type ChatEvent struct {
	Speaker string
	Message string
	Raw     string
	Kinds   int
}

type ServerMessageFilter struct {
	Contains string
	Type     string
}

type ServerMessageEvent struct {
	Message string
	Type    string
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

// scriptRegisterChat registers a chat trigger with optional name and kind flags.
func scriptRegisterChat(owner, name string, phrases []string, flags int, fn func(string)) {
	if scriptIsDisabled(owner) || fn == nil {
		return
	}
	var registration scriptRegistrationHandle
	registration = registerScriptResource(owner, func() {
		triggerHandlersMu.Lock()
		for phrase, handlers := range scriptTriggers {
			kept := handlers[:0]
			for _, handler := range handlers {
				if handler.registration != registration {
					kept = append(kept, handler)
				}
			}
			if len(kept) == 0 {
				delete(scriptTriggers, phrase)
			} else {
				scriptTriggers[phrase] = kept
			}
		}
		triggerHandlersMu.Unlock()
	})
	triggerHandlersMu.Lock()
	name = strings.ToLower(name)
	for _, p := range phrases {
		if p == "" {
			continue
		}
		p = strings.ToLower(p)
		scriptTriggers[p] = append(scriptTriggers[p], triggerHandler{owner: owner, name: name, flags: flags, fn: fn, queue: currentScriptEventQueue(owner), registration: registration})
	}
	triggerHandlersMu.Unlock()
	refreshTriggersList()
}

// Back-compat wrapper for older API without flags.
func scriptRegisterTriggers(owner, name string, phrases []string, fn func()) {
	if fn == nil {
		return
	}
	scriptRegisterChat(owner, name, phrases, ChatAny, func(string) { fn() })
}

// New console registration with message parameter
func scriptRegisterConsole(owner string, phrases []string, fn func(string)) {
	if scriptIsDisabled(owner) || fn == nil {
		return
	}
	var registration scriptRegistrationHandle
	registration = registerScriptResource(owner, func() {
		triggerHandlersMu.Lock()
		for phrase, handlers := range scriptConsoleTriggers {
			kept := handlers[:0]
			for _, handler := range handlers {
				if handler.registration != registration {
					kept = append(kept, handler)
				}
			}
			if len(kept) == 0 {
				delete(scriptConsoleTriggers, phrase)
			} else {
				scriptConsoleTriggers[phrase] = kept
			}
		}
		triggerHandlersMu.Unlock()
	})
	triggerHandlersMu.Lock()
	for _, p := range phrases {
		if p == "" {
			continue
		}
		p = strings.ToLower(p)
		scriptConsoleTriggers[p] = append(scriptConsoleTriggers[p], triggerHandler{owner: owner, fn: fn, queue: currentScriptEventQueue(owner), registration: registration})
	}
	triggerHandlersMu.Unlock()
	refreshTriggersList()
}

// Back-compat: old console registration without msg parameter
func scriptRegisterConsoleTriggers(owner string, phrases []string, fn func()) {
	if fn == nil {
		return
	}
	scriptRegisterConsole(owner, phrases, func(string) { fn() })
}

func scriptRegisterPlayerHandler(owner string, fn func(Player)) {
	if scriptIsDisabled(owner) || fn == nil {
		return
	}
	var registration scriptRegistrationHandle
	registration = registerScriptResource(owner, func() {
		playerHandlersMu.Lock()
		for i := len(scriptPlayerHandlers) - 1; i >= 0; i-- {
			if scriptPlayerHandlers[i].registration == registration {
				scriptPlayerHandlers = append(scriptPlayerHandlers[:i], scriptPlayerHandlers[i+1:]...)
			}
		}
		playerHandlersMu.Unlock()
	})
	playerHandlersMu.Lock()
	scriptPlayerHandlers = append(scriptPlayerHandlers, playerHandler{owner: owner, fn: fn, queue: currentScriptEventQueue(owner), registration: registration})
	playerHandlersMu.Unlock()
}

// scriptRegisterChatHandler registers a callback invoked for every chat message.
func scriptRegisterChatHandler(owner string, fn func(string)) {
	if scriptIsDisabled(owner) || fn == nil {
		return
	}
	var registration scriptRegistrationHandle
	registration = registerScriptResource(owner, func() {
		chatHandlersMu.Lock()
		for i := len(scriptChatHandlers) - 1; i >= 0; i-- {
			if scriptChatHandlers[i].registration == registration {
				scriptChatHandlers = append(scriptChatHandlers[:i], scriptChatHandlers[i+1:]...)
			}
		}
		chatHandlersMu.Unlock()
	})
	chatHandlersMu.Lock()
	scriptChatHandlers = append(scriptChatHandlers, chatHandler{owner: owner, fn: fn, queue: currentScriptEventQueue(owner), registration: registration})
	chatHandlersMu.Unlock()
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

func scriptRegisterServerMessage(owner string, filter ServerMessageFilter, fn func(ServerMessageEvent)) scriptRegistrationHandle {
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

func runInputHandlers(txt string) string {
	inputHandlersMu.RLock()
	handlers := append([]inputHandler{}, scriptInputHandlers...)
	inputHandlersMu.RUnlock()
	for _, h := range handlers {
		if h.fn != nil {
			scriptLogEvent(h.owner, "InputHandler", txt)
			next := txt
			if queueScriptCallbackWaitOn(h.queue, h.owner, "InputHandler", func() { next = h.fn(txt) }) {
				txt = next
			}
		}
	}
	return txt
}

func runChatTriggers(msg string) {
	triggerHandlersMu.RLock()
	event := classifyScriptChat(msg)
	speaker := event.Speaker
	msgLower := strings.ToLower(msg)
	speakerLower := strings.ToLower(speaker)
	msgFlags := event.Kinds
	for phrase, hs := range scriptTriggers {
		if strings.Contains(msgLower, phrase) {
			for _, h := range hs {
				if h.name != "" && h.name != speakerLower {
					continue
				}
				f := h.flags
				if f == 0 {
					f = ChatAny
				}
				if (f & msgFlags) != 0 {
					owner := h.owner
					fn := h.fn
					scriptLogEvent(owner, "ChatTrigger", fmt.Sprintf("%q %q", phrase, msg))
					queueScriptCallbackOn(h.queue, owner, "ChatTrigger", func() { fn(msg) })
				}
			}
		}
	}
	triggerHandlersMu.RUnlock()

	// Dispatch all-chat handlers (no phrase filtering).
	chatHandlersMu.RLock()
	handlers := append([]chatHandler{}, scriptChatHandlers...)
	structuredHandlers := append([]structuredChatHandler{}, scriptStructuredChatHandlers...)
	chatHandlersMu.RUnlock()
	for _, h := range handlers {
		if h.fn != nil {
			scriptLogEvent(h.owner, "ChatHandler", msg)
			queueScriptCallbackOn(h.queue, h.owner, "ChatHandler", func() { h.fn(msg) })
		}
	}
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
	event := ChatEvent{Speaker: speaker, Message: scriptChatMessage(msg, speaker), Raw: msg, Kinds: ChatAny}
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

func scriptChatMessage(raw, speaker string) string {
	message := strings.TrimSpace(raw)
	if strings.HasPrefix(message, "(") {
		if end := strings.IndexByte(message, ')'); end >= 0 {
			return strings.TrimSpace(message[end+1:])
		}
	}
	lower := strings.ToLower(message)
	for _, separator := range []string{" says", " yells", " whispers", " asks", " exclaims"} {
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
	if filter.Speaker != "" && !strings.EqualFold(strings.TrimSpace(filter.Speaker), event.Speaker) {
		return false
	}
	if filter.Contains != "" && !strings.Contains(strings.ToLower(event.Message), strings.ToLower(filter.Contains)) {
		return false
	}
	return filter.Kinds == 0 || filter.Kinds&event.Kinds != 0
}

func runServerMessageHandlers(event ServerMessageEvent) {
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

func runConsoleTriggers(msg string) {
	triggerHandlersMu.RLock()
	msgLower := strings.ToLower(msg)
	for phrase, hs := range scriptConsoleTriggers {
		if strings.Contains(msgLower, phrase) {
			for _, h := range hs {
				scriptLogEvent(h.owner, "ConsoleTrigger", fmt.Sprintf("%q %q", phrase, msg))
				fn := h.fn
				owner := h.owner
				queueScriptCallbackOn(h.queue, owner, "ConsoleTrigger", func() { fn(msg) })
			}
		}
	}
	triggerHandlersMu.RUnlock()
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
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !isUserScriptFile(e.Name()) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			filePath := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			snapshot[filePath] = scriptFileState{
				size:    info.Size(),
				modTime: info.ModTime().UnixNano(),
				sum:     sha256.Sum256(data),
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
	name        string
	author      string
	category    string
	subCategory string
	path        string
	src         []byte
	invalid     bool
	apiVer      int
}

func scanscripts(scriptDirs []string, dup func(name, path string)) map[string]scriptInfo {
	nameRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+scriptName\s*=\s*"([^"]+)"`)
	authorRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+scriptAuthor\s*=\s*"([^"]+)"`)
	categoryRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+scriptCategory\s*=\s*"([^"]+)"`)
	subCategoryRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+scriptSubCategory\s*=\s*"([^"]+)"`)
	apiVerRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+scriptAPIVersion\s*=\s*([0-9]+)\s*$`)
	scripts := map[string]scriptInfo{}
	seenNames := map[string]bool{}
	for _, dir := range scriptDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Printf("read script dir %s: %v", dir, err)
			}
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !isUserScriptFile(e.Name()) {
				continue
			}
			path := filepath.Join(dir, e.Name())
			src, err := os.ReadFile(path)
			if err != nil {
				log.Printf("read script %s: %v", path, err)
				continue
			}
			nameMatch := nameRE.FindSubmatch(src)
			base := strings.TrimSuffix(e.Name(), ".go")
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
			invalid := false
			apiVer := 0
			if len(nameMatch) < 2 || name == "" || invalidscriptValue(name) {
				if len(nameMatch) < 2 || name == "" {
					consoleMessage("[script] missing name: " + path)
					name = base
				} else {
					consoleMessage("[script] invalid name: " + path)
				}
				invalid = true
			}
			if author == "" || invalidscriptValue(author) {
				if author == "" {
					consoleMessage("[script] missing author: " + path)
				} else {
					consoleMessage("[script] invalid author: " + path)
				}
				invalid = true
			}
			if category == "" || invalidscriptValue(category) {
				if category == "" {
					consoleMessage("[script] missing category: " + path)
				} else {
					consoleMessage("[script] invalid category: " + path)
				}
				invalid = true
			}
			if m := apiVerRE.FindSubmatch(src); len(m) >= 2 {
				if n, err := strconv.Atoi(strings.TrimSpace(string(m[1]))); err == nil {
					apiVer = n
				}
			}
			lower := strings.ToLower(name)
			if seenNames[lower] {
				if dup != nil {
					dup(name, path)
				}
				continue
			}
			seenNames[lower] = true
			owner := name + "_" + base
			scripts[owner] = scriptInfo{
				name:        name,
				author:      author,
				category:    category,
				subCategory: subCategory,
				path:        path,
				src:         src,
				invalid:     invalid,
				apiVer:      apiVer,
			}
		}
	}
	return scripts
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
		}
	}

	reloadOwners := make([]string, 0, len(scanned))
	scriptMu.Lock()
	scriptDisplayNames = make(map[string]string, len(scanned))
	scriptPaths = make(map[string]string, len(scanned))
	scriptAuthors = make(map[string]string, len(scanned))
	scriptCategories = make(map[string]string, len(scanned))
	scriptSubCategories = make(map[string]string, len(scanned))
	scriptInvalid = make(map[string]bool, len(scanned))
	scriptDisabled = make(map[string]bool, len(scanned))
	newEnabled := map[string]scriptScope{}
	for o, info := range scanned {
		scriptDisplayNames[o] = info.name
		scriptPaths[o] = info.path
		scriptAuthors[o] = info.author
		scriptCategories[o] = info.category
		scriptSubCategories[o] = info.subCategory
		if en, ok := scriptEnabledFor[o]; ok {
			newEnabled[o] = en
		} else if gs.Enabledscripts != nil {
			if val, ok := gs.Enabledscripts[o]; ok {
				newEnabled[o] = scopeFromSettingValue(val)
			}
		}
		// Require a matching script API version
		invalid := info.invalid || info.apiVer != scriptAPICurrentVersion
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
		if oldRunning[o] && shouldEnable {
			reloadOwners = append(reloadOwners, o)
		}
	}
	scriptEnabledFor = newEnabled
	scriptNames = make(map[string]bool, len(scanned))
	for _, info := range scanned {
		scriptNames[strings.ToLower(info.name)] = true
	}
	scriptMu.Unlock()

	sort.Strings(reloadOwners)
	for _, owner := range reloadOwners {
		info := scanned[owner]
		loadscriptSource(owner, info.name, info.path, info.src, restrictedStdlib())
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
	ensureDefaultScripts()
	scanned := scanscripts(scriptSearchDirs(), func(name, path string) {
		log.Printf("script %s duplicate name %s", path, name)
		consoleMessage("[script] duplicate name: " + name)
	})

	scriptNames = make(map[string]bool, len(scanned))
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
		scriptPaths[o] = info.path
		if !s.empty() {
			scriptEnabledFor[o] = s
		}
		scriptAuthors[o] = info.author
		scriptInvalid[o] = invalid
		scriptDisabled[o] = disabled
		scriptMu.Unlock()
		if !disabled {
			loadscriptSource(o, info.name, info.path, info.src, restrictedStdlib())
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
