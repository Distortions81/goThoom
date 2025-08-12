package eui

import "log"

// UIEventType defines the kind of event emitted by widgets.
type UIEventType int

const (
	EventClick UIEventType = iota
	EventSliderChanged
	EventDropdownSelected
	EventCheckboxChanged
	EventRadioSelected
	EventColorChanged
	EventInputChanged
	EventInputSubmit
)

// UIEvent describes a user interaction with a widget.
type UIEvent struct {
	Item    *ItemData
	Type    UIEventType
	Value   float32
	Index   int
	Checked bool
	Color   Color
	Text    string
}

// EventHandler holds a channel widgets use to emit events.
// EventHandler provides both channel and callback based event delivery.
type EventHandler struct {
	Events chan UIEvent
	Handle func(UIEvent)
}

// Emit delivers the event through the channel and callback if present. If the
// channel is full the event is dropped and logged rather than blocking.
func (h *EventHandler) Emit(ev UIEvent) {
	if h == nil {
		return
	}
	if h.Events != nil {
		select {
		case h.Events <- ev:
		default:
			log.Printf("event channel full, dropping event: %v", ev.Type)
		}
	}
	if h.Handle != nil {
		h.Handle(ev)
	}
}

func newHandler() *EventHandler {
	return &EventHandler{Events: make(chan UIEvent, 64)}
}
