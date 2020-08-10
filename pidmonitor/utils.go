//
// Copyright 2019-2020 Nestybox, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

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

func eventTableAdd(t map[uint32]int, e PidEvent) {
	pid := e.Pid
	pidEvent := e.Event

	evect, found := t[pid]
	if !found {
		t[pid] = pidEvent
	} else {
		t[pid] = eventSet(evect, pidEvent)
	}
}

func eventTableRm(t map[uint32]int, e PidEvent) {
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
