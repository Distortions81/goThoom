package main

import (
	"sync"
	"time"
)

type Task struct{ state *scriptTaskState }
type scriptTaskState struct {
	mu                           sync.Mutex
	owner                        string
	commands                     *commandState
	done                         chan struct{}
	cancelled, finished, started bool
}
type scriptTaskCancelled struct{}

func newScriptTask(owner string) Task {
	return Task{&scriptTaskState{owner: owner, done: make(chan struct{})}}
}
func (t Task) Active() bool {
	if t.state == nil {
		return false
	}
	t.state.mu.Lock()
	defer t.state.mu.Unlock()
	return t.state.started && !t.state.cancelled && !t.state.finished
}
func (t Task) Cancel() {
	if t.state == nil {
		return
	}
	s := t.state
	s.mu.Lock()
	if !s.cancelled {
		s.cancelled = true
		close(s.done)
	}
	commands := s.commands
	s.mu.Unlock()
	if commands == nil {
		commands = primarySession.commands
	}
	commands.cancelScriptCommands(s.owner, s)
}
func (t Task) start(queue *scriptEventQueue, fn func()) {
	s := t.state
	if queue == nil {
		t.Cancel()
		return
	}
	s.mu.Lock()
	if s.cancelled {
		s.mu.Unlock()
		return
	}
	s.started = true
	if queue.session != nil {
		s.commands = queue.session.commands
	} else {
		s.commands = primarySession.commands
	}
	s.mu.Unlock()
	handle := registerScriptResourceOn(queue, func() {
		if t.Active() {
			t.Cancel()
		}
	})
	if !queue.enqueue(scriptEvent{name: "Task", task: s, callback: func() {
		defer func() {
			s.mu.Lock()
			s.finished = true
			s.mu.Unlock()
			handle.release()
		}()
		checkScriptTask(queue)
		fn()
	}}) {
		t.Cancel()
		handle.release()
	}
}
func (q *scriptEventQueue) task() *scriptTaskState {
	if q == nil {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.activeTask
}
func checkScriptTask(q *scriptEventQueue) {
	if s := q.task(); s != nil {
		select {
		case <-s.done:
			panic(scriptTaskCancelled{})
		default:
		}
	}
}

// Waits in an explicit task lend execution to ordinary callbacks, never to
// another task. This keeps interpreter access serialized and bounds nesting.
func waitScriptSignal(q *scriptEventQueue, signal <-chan struct{}, timeout <-chan time.Time) bool {
	task := q.task()
	if task == nil {
		select {
		case <-signal:
			return true
		case <-timeout:
			return false
		case <-q.done:
			return false
		}
	}
	for {
		checkScriptTask(q)
		select {
		case <-task.done:
			panic(scriptTaskCancelled{})
		case <-q.done:
			panic(scriptTaskCancelled{})
		case <-signal:
			return true
		case <-timeout:
			return false
		case <-q.wake:
			// Run ordinary callbacks while this task waits, checking timeouts and
			// cancellation between callbacks. Other tasks keep their queue order.
			for {
				checkScriptTask(q)
				select {
				case <-signal:
					return true
				case <-timeout:
					return false
				case <-q.done:
					panic(scriptTaskCancelled{})
				default:
				}
				q.mu.Lock()
				index := -1
				for i, event := range q.events {
					if event.task == nil {
						index = i
						break
					}
				}
				if index < 0 {
					q.mu.Unlock()
					break
				}
				event := q.events[index]
				copy(q.events[index:], q.events[index+1:])
				q.events[len(q.events)-1] = scriptEvent{}
				q.events = q.events[:len(q.events)-1]
				q.mu.Unlock()
				ok := q.execute(event)
				if event.done != nil {
					event.done <- ok
				}
				if !ok {
					q.stop()
					panic(scriptTaskCancelled{})
				}
			}
		}
	}
}
