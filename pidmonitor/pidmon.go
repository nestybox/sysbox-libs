// The pidmon package allows a process to get notificaitons on events associated with
// other processes.

package pidmonitor

import (
	"fmt"
	"sync"
	"time"
)

// pidMon configuration info
type Cfg struct {
	Poll time.Duration // polling time, in ms
}

// polling config limits (in ms)
const (
	PollMin = 1
	PollMax = 1000
)

// Pid event types (bit-vector)
const (
	Exit int = 0x1 // Process exited
)

// Represents an event on the given process
type PidEvent struct {
	Pid   int
	Event int   // bit vector of events
	Err   error // set by WaitEvent() when an error is detected
}

// Represents a pid monitor instance
type PidMon struct {
	mu         sync.Mutex
	cfg        *Cfg
	eventTable map[int]int     // maps each pid to it's event vector
	cmdCh      chan cmd        // sends commands to monitor thread
	eventCh    chan []PidEvent // receives events from monitor thread
}

// Creates a instance of the pid monitor; returns a the pidMon ID.
func New(cfg *Cfg) (*PidMon, error) {

	if err := validateCfg(cfg); err != nil {
		return nil, err
	}

	pm := &PidMon{
		cfg:        cfg,
		eventTable: make(map[int]int),
		cmdCh:      make(chan cmd),
		eventCh:    make(chan []PidEvent, 10), // buffered to prevent monitor thread from blocking when pushing events
	}

	go pidMonitor(pm)

	return pm, nil
}

// Adds one or more events to the list of events monitored by the given pidMon
func (pm *PidMon) AddEvent(events []PidEvent) error {

	for _, e := range events {
		if !validateEvent(e.Event) {
			return fmt.Errorf("Unknown event %v", e.Event)
		}
		pm.mu.Lock()
		eventTableAdd(pm.eventTable, e)
		pm.mu.Unlock()
	}

	return nil
}

// Removes one or more events from the list of events monitored by the given pidMon
func (pm *PidMon) RemoveEvent(events []PidEvent) error {

	for _, e := range events {
		if !validateEvent(e.Event) {
			return fmt.Errorf("Unknown event %v", e.Event)
		}
		pm.mu.Lock()
		eventTableRm(pm.eventTable, e)
		pm.mu.Unlock()
	}

	return nil
}

// Blocks the calling process until the given pidMon detects an event in one or more of
// the processes it's monitoring. Returns the list of events.
func (pm *PidMon) WaitEvent() []PidEvent {
	eventList := <-pm.eventCh
	return eventList
}

// Stops the given pidMon. Causes WaitEvent() to return immediately (likely
// with an empty pid list).
func (pm *PidMon) Close() {
	pm.cmdCh <- stop
}
