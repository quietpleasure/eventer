package eventer

import (
	"context"
	"fmt"
	"time"

	"github.com/gookit/event"
)

type EventManager struct {
	manager *event.Manager
}

type Event struct {
	event.BasicEvent
}

type Reply struct {
	Error   error
	Message string
}

type EventListner interface {
	event.Listener
	Name() string
}

const default_name_manager = "manager"

func NewManager(name ...string) *EventManager {
	mngrName := default_name_manager
	if len(name) > 0 {
		mngrName = name[0]
	}
	return &EventManager{
		manager: event.NewManager(mngrName),
	}
}

func (em *EventManager) Close() error {
	return em.manager.Close()
}

func (em *EventManager) RegisterEventListeners(listeners ...event.Listener) {
	for _, listener := range listeners {
		em.manager.AddListener(fmt.Sprintf("%s", listener), listener)
	}
}

type ListenerData struct {
	Listener EventListner
	Data     map[string]any
}

func NewEventsConsumers(listdata []ListenerData) []*Event {
	var events []*Event
	for _, ld := range listdata {
		e := new(Event)
		e.SetName(ld.Listener.Name())
		e.SetData(ld.Data)
		e.SetTarget(ld.Listener)
		events = append(events, e)
	}
	return events
}

func NewEventConsumer(listener EventListner, data map[string]any) *Event {
	e := new(Event)
	e.SetName(listener.Name())
	e.SetData(data)
	e.SetTarget(listener)
	return e
}

func (em *EventManager) FireEvent(ctx context.Context, event *Event, reply chan Reply) {
	feedback := make(chan Reply)
	defer close(feedback)
	retrying := retryFire(ctx, em.manager.FireEvent, feedback)
	go retrying(event)
	for r := range feedback {
		reply <- r
		if r.Error == nil {
			return
		}
		continue
	}
}

type fireraiser func(event event.Event) error //(em *Manager) FireEvent(e Event) (err error)

func retryFire(ctx context.Context, fireraiser fireraiser, feedback chan Reply) fireraiser {
	return func(evnt event.Event) error {
		for try := 1; ; try++ {
			err := fireraiser(evnt)
			if err == nil {
				feedback <- Reply{
					Message: fmt.Sprintf("successful handle [%s] event", evnt.Name()),
				}
				return nil
			}

			delay := time.Second << uint(try)
			feedback <- Reply{
				Error:   err,
				Message: fmt.Sprintf("[%s] failed attempt [%d] | retrying after: [%s]", evnt.Name(), try, delay),
			}
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				feedback <- Reply{
					Error:   ctx.Err(),
					Message: fmt.Sprintf("retrying event [%s] stopped via context", evnt.Name()),
				}
				return ctx.Err()
			}
		}
	}
}
