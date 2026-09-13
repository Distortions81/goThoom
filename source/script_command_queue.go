package main

import (
	"strings"

	scriptapi "gt2"
)

// Ticket state and queue metadata share their session command-state lock,
// making cancellation atomic with selecting a command for a network write.
type CommandTicket struct{ state *scriptCommandState }
type scriptCommandState struct {
	candidate       *scriptCandidate
	cancelRequested bool
	commands        *commandState
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

func newScriptCommandTicket(owner string, queue *scriptEventQueue) CommandTicket {
	commands := primarySession.commands
	if queue != nil && queue.session != nil {
		commands = queue.session.commands
	}
	return newSessionScriptCommandTicket(commands, owner, queue)
}
func newSessionScriptCommandTicket(commands *commandState, owner string, queue *scriptEventQueue) CommandTicket {
	return CommandTicket{&scriptCommandState{commands: commands, owner: owner, queue: queue, task: queue.task(), status: scriptapi.CommandStatus{State: scriptapi.CommandQueued}}}
}
func (t CommandTicket) Status() scriptapi.CommandStatus {
	if t.state != nil && t.state.candidate != nil && !t.state.candidate.apiCallAllowed(t.state.owner, "send") {
		t.Cancel()
	}
	if t.state == nil || t.state.commands == nil {
		return scriptapi.CommandStatus{State: scriptapi.CommandRejected, Reason: "unavailable"}
	}
	t.state.commands.mu.Lock()
	defer t.state.commands.mu.Unlock()
	return t.state.status
}
func (t CommandTicket) Cancel() bool {
	if t.state == nil {
		return false
	}
	if t.state.commands == nil {
		return false
	}
	t.state.commands.mu.Lock()
	defer t.state.commands.mu.Unlock()
	return t.state.commands.cancelTicketLocked(t.state)
}
func (s *commandState) cancelTicketLocked(ticket *scriptCommandState) bool {
	if ticket == nil || ticket.status.State != scriptapi.CommandQueued {
		return false
	}
	if ticket.inFlight {
		ticket.cancelRequested = true
		return false
	}
	if s.pendingTicket == ticket {
		// A write already in progress cannot be recalled, including retransmissions.
		if s.pendingSent {
			ticket.cancelRequested = true
			return false
		}
		s.pending, s.pendingID, s.pendingTicket = "", 0, nil
		s.resetPendingTimingLocked()
	}
	for i := 0; i < len(s.queue); i++ {
		if s.queue[i].ticket == ticket {
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			i--
		}
	}
	ticket.status = scriptapi.CommandStatus{State: scriptapi.CommandCancelled}
	s.nextLocked()
	return true
}
func cancelScriptCommands(owner string, task *scriptTaskState) {
	primarySession.commands.cancelScriptCommands(owner, task)
}
func (s *commandState) cancelScriptCommands(owner string, task *scriptTaskState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var tickets []*scriptCommandState
	matches := func(t *scriptCommandState) bool {
		return t != nil && t.owner == owner && (task == nil || t.task == task)
	}
	if matches(s.pendingTicket) {
		tickets = append(tickets, s.pendingTicket)
	}
	for _, cmd := range s.queue {
		if matches(cmd.ticket) {
			tickets = append(tickets, cmd.ticket)
		}
	}
	for _, ticket := range tickets {
		s.cancelTicketLocked(ticket)
	}
}
func queueTrackedScriptCommand(ticket CommandTicket, cmd string) bool {
	if ticket.state == nil || ticket.state.commands == nil {
		return false
	}
	return ticket.state.commands.queueTrackedScriptCommand(ticket, cmd)
}
func (s *commandState) queueTrackedScriptCommand(ticket CommandTicket, cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if ticket.state.queue == nil {
		ticket.state.queue = currentScriptEventQueue(ticket.state.owner)
	}
	reason := ""
	if scriptRuntimeDisabled(ticket.state.owner, ticket.state.queue) {
		reason = "script stopped"
	} else if cmd == "" {
		reason = "empty command"
	} else if recordscriptSendOn(ticket.state.owner, ticket.state.queue) {
		reason = "rate limit exceeded"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.queue = append(s.queue, queuedCommand{text: cmd, ticket: ticket.state})
	s.nextLocked()
	return true
}
