package pidmonitor

import "fmt"

func validateCfg(cfg *Cfg) error {
	if cfg.Poll < PollMin || cfg.Poll > PollMax {
		return fmt.Errorf("invalid config: poll time must be in range [%d, %d]; found %d", PollMin, PollMax, cfg.Poll)
	}
	return nil
}

func validateEvent(event int) bool {
	return event == Exit
}

func eventSet(evect int, etype int) int {
	return evect | etype
}

func eventClear(evect int, etype int) int {
	return evect &^ etype
}

func eventIsSet(evect int, etype int) bool {
	return evect&etype == etype
}

func eventTableAdd(t map[int]int, e PidEvent) {
	pid := e.Pid
	pidEvent := e.Event

	evect, found := t[pid]
	if !found {
		t[pid] = pidEvent
	} else {
		t[pid] = eventSet(evect, pidEvent)
	}
}

func eventTableRm(t map[int]int, e PidEvent) {
	pid := e.Pid
	pidEvent := e.Event

	evect, found := t[pid]
	if found {
		evect = eventClear(evect, pidEvent)
		if evect == 0 {
			delete(t, pid)
		} else {
			t[pid] = evect
		}
	}
}
