package main

import (
	"fmt"
	scriptapi "gt2"
	"testing"
)

func TestScriptCommandTickets(t *testing.T) {
	const owner = "command-ticket-test"
	resetScriptCallbackTestState(t, owner)
	resetCommandStateForTest(t, 1)
	clearCommands()
	t.Cleanup(func() { clearCommands(); stopScriptEventQueue(owner) })
	queue := exportsForscript(owner)["gt2/gt2"]["QueueCommand"].Interface().(func(string) CommandTicket)
	rejected := queue("  ")
	if status := rejected.Status(); status.State != scriptapi.CommandRejected || status.Reason != "empty command" {
		t.Fatalf("empty command: %+v", status)
	}
	first, second := queue("/pose sit"), queue("/pose kneel")
	enqueueCommand("/manual")
	if !second.Cancel() || second.Cancel() {
		t.Fatal("ticket cancellation is not idempotent")
	}
	if err := sendPlayerInput(&writeErrorConn{}, 0, 0, false, false); err == nil {
		t.Fatal("write should fail")
	}
	if first.Status().State != scriptapi.CommandQueued {
		t.Fatal("failed write reported sent")
	}
	if err := sendPlayerInput(&bufConn{}, 0, 0, false, false); err != nil {
		t.Fatal(err)
	}
	if first.Status().State != scriptapi.CommandSent || first.Cancel() {
		t.Fatal("sent ticket can be cancelled")
	}
	acknowledgeCommand(primarySession.commands.pendingID+1, 0)
	if first.Cancel() {
		t.Fatal("retransmission can be cancelled after sending")
	}
	acknowledgeCommand(primarySession.commands.pendingID, 0)
	if primarySession.commands.pending != "/manual" {
		t.Fatalf("manual command order: %q", primarySession.commands.pending)
	}
	third := queue("/pose stand")
	cancelScriptCommands(owner, nil)
	if third.Status().State != scriptapi.CommandCancelled || primarySession.commands.pending != "/manual" {
		t.Fatal("script cleanup affected manual command")
	}
	clearCommands()
	fourth := queue("/pose sit")
	clearCommands()
	if fourth.Status().State != scriptapi.CommandCancelled {
		t.Fatal("disconnect left queued ticket")
	}
	scriptPermissionMu.Lock()
	scriptPermissionGrants[owner]["send"] = false
	scriptPermissionMu.Unlock()
	if queue("/denied").Status().State != scriptapi.CommandRejected {
		t.Fatal("denied send returned accepted ticket")
	}
}
func TestScriptTaskCancellationKeepsOtherCommands(t *testing.T) {
	const owner = "command-task-ownership"
	resetScriptCallbackTestState(t, owner)
	clearCommands()
	t.Cleanup(func() { clearCommands(); stopScriptEventQueue(owner) })
	q := currentScriptEventQueue(owner)
	job := newScriptTask(owner)
	q.mu.Lock()
	q.activeTask = job.state
	q.mu.Unlock()
	first := newScriptCommandTicket(owner, q)
	queueTrackedScriptCommand(first, "/task-first")
	second := newScriptCommandTicket(owner, q)
	queueTrackedScriptCommand(second, "/task-second")
	q.mu.Lock()
	q.activeTask = nil
	q.mu.Unlock()
	other := newScriptCommandTicket(owner, q)
	queueTrackedScriptCommand(other, "/other")
	enqueueCommand("/manual")
	job.Cancel()
	if first.Status().State != scriptapi.CommandCancelled || second.Status().State != scriptapi.CommandCancelled || other.Status().State != scriptapi.CommandQueued {
		t.Fatal("task ownership lost")
	}
	if primarySession.commands.pending != "/other" || len(primarySession.commands.queue) != 1 || primarySession.commands.queue[0].text != "/manual" {
		t.Fatal("cancellation reordered other commands")
	}
}

// Cancel during the actual network write to exercise ownership at the boundary
// between queued and sent, including a reply arriving before Write returns.
type cancellingCommandConn struct {
	bufConn
	duringWrite func()
	fail        bool
}

func (c *cancellingCommandConn) Write(b []byte) (int, error) {
	c.duringWrite()
	if c.fail {
		return (&writeErrorConn{}).Write(b)
	}
	return c.bufConn.Write(b)
}
func TestScriptCommandCancellationDuringNetworkWrite(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			const owner = "command-inflight-test"
			resetScriptCallbackTestState(t, owner)
			resetCommandStateForTest(t, 1)
			clearCommands()
			t.Cleanup(func() { clearCommands(); stopScriptEventQueue(owner) })
			ticket := newScriptCommandTicket(owner, currentScriptEventQueue(owner))
			queueTrackedScriptCommand(ticket, "/pose sit")
			conn := &cancellingCommandConn{fail: fail, duringWrite: func() {
				acknowledgeCommand(primarySession.commands.pendingID+1, 0)
				if ticket.Cancel() {
					t.Fatal("cancel recalled an in-flight write")
				}
			}}
			err := sendPlayerInput(conn, 0, 0, false, false)
			if fail {
				if err == nil || ticket.Status().State != scriptapi.CommandCancelled || !commandQueueIsIdle() {
					t.Fatal("cancelled failed write remained for retry")
				}
			} else if err != nil || ticket.Status().State != scriptapi.CommandSent {
				t.Fatal("successful in-flight write did not report sent")
			}
		})
	}
}
