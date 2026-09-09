package main

import (
	"strings"

	scriptapi "gt2"
)

// Ticket state and queue metadata share commandMu, making cancellation atomic
// with selecting a command for a network write.
type CommandTicket struct{ state *scriptCommandState }
type scriptCommandState struct {
	candidate       *scriptCandidate
	cancelRequested bool
	inFlight        bool
	queue           *scriptEventQueue
	owner           string
	task            *scriptTaskState
	status          scriptapi.CommandStatus
}
type queuedCommand struct {
	text   string
	ticket *scriptCommandState
}

var pendingCommandTicket *scriptCommandState

func newScriptCommandTicket(owner string, queue *scriptEventQueue) CommandTicket {
	return CommandTicket{&scriptCommandState{owner: owner, queue: queue, task: queue.task(), status: scriptapi.CommandStatus{State: scriptapi.CommandQueued}}}
}
func (t CommandTicket) Status() scriptapi.CommandStatus {
	if t.state != nil && t.state.candidate != nil && !t.state.candidate.apiCallAllowed(t.state.owner, "send") {
		t.Cancel()
	}
	commandMu.Lock()
	defer commandMu.Unlock()
	if t.state == nil {
		return scriptapi.CommandStatus{State: scriptapi.CommandRejected, Reason: "unavailable"}
	}
	return t.state.status
}
func (t CommandTicket) Cancel() bool {
	if t.state == nil {
		return false
	}
	commandMu.Lock()
	defer commandMu.Unlock()
	return cancelCommandTicketLocked(t.state)
}
func cancelCommandTicketLocked(ticket *scriptCommandState) bool {
	if ticket == nil || ticket.status.State != scriptapi.CommandQueued {
		return false
	}
	if ticket.inFlight {
		ticket.cancelRequested = true
		return false
	}
	if pendingCommandTicket == ticket {
		// A write already in progress cannot be recalled, including retransmissions.
		if pendingCommandSent {
			ticket.cancelRequested = true
			return false
		}
		pendingCommand, pendingCommandID, pendingCommandTicket = "", 0, nil
		resetPendingCommandTimingLocked()
	}
	for i := 0; i < len(commandQueue); i++ {
		if commandQueue[i].ticket == ticket {
			commandQueue = append(commandQueue[:i], commandQueue[i+1:]...)
			i--
		}
	}
	ticket.status = scriptapi.CommandStatus{State: scriptapi.CommandCancelled}
	nextCommandLocked()
	return true
}
func cancelScriptCommands(owner string, task *scriptTaskState) {
	commandMu.Lock()
	defer commandMu.Unlock()
	var tickets []*scriptCommandState
	matches := func(t *scriptCommandState) bool {
		return t != nil && t.owner == owner && (task == nil || t.task == task)
	}
	if matches(pendingCommandTicket) {
		tickets = append(tickets, pendingCommandTicket)
	}
	for _, cmd := range commandQueue {
		if matches(cmd.ticket) {
			tickets = append(tickets, cmd.ticket)
		}
	}
	for _, ticket := range tickets {
		cancelCommandTicketLocked(ticket)
	}
}
func queueTrackedScriptCommand(ticket CommandTicket, cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if ticket.state.queue == nil {
		ticket.state.queue = currentScriptEventQueue(ticket.state.owner)
	}
	reason := ""
	if scriptIsDisabled(ticket.state.owner) {
		reason = "script stopped"
	} else if cmd == "" {
		reason = "empty command"
	} else if recordscriptSend(ticket.state.owner) {
		reason = "rate limit exceeded"
	}
	commandMu.Lock()
	defer commandMu.Unlock()
	if ticket.state.status.State != scriptapi.CommandQueued {
		return false
	}
	if task := ticket.state.task; task != nil {
		select {
		case <-task.done:
			ticket.state.status.State = scriptapi.CommandCancelled
			return false
		default:
		}
	}
	if queue := ticket.state.queue; queue != nil {
		queue.mu.Lock()
		stopped := queue.stopped
		queue.mu.Unlock()
		if stopped && reason == "" {
			reason = "script stopped"
		}
	}
	if reason != "" {
		ticket.state.status = scriptapi.CommandStatus{State: scriptapi.CommandRejected, Reason: reason}
		return false
	}
	commandQueue = append(commandQueue, queuedCommand{text: cmd, ticket: ticket.state})
	nextCommandLocked()
	return true
}
