//
// Copyright 2023 Nestybox, Inc.
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

package fileMonitor

import (
	"os"
	"time"
)

type cmd int

const (
	stop cmd = iota
)

// Monitors files associated with the given FileMon instance
func fileMon(fm *FileMon) {
	for {
		eventList := []Event{}
		rmList := []Event{}

		// handle incoming commands first
		select {
		case cmd := <-fm.cmdCh:
			if cmd == stop {
				fm.eventCh <- eventList
				return
			}
		default:
		}

		// perform monitoring action
		fm.mu.Lock()
		for filename, _ := range fm.eventTable {
			exists, err := checkFileExists(filename)
			if err != nil || !exists {
				eventList = append(eventList, Event{
					Filename: filename,
					Err:      err,
				})

				// file removal implies event won't hit again; remove it.
				rmList = append(rmList, Event{filename, nil})
			}
		}

		// release the lock so that we don't hold it while sending the event list
		// (in case the event channel is blocked); this way new events can
		// continue to be added.
		fm.mu.Unlock()

		// send event list
		if len(eventList) > 0 {
			fm.eventCh <- eventList
		}

		// remove events that won't hit any more
		fm.mu.Lock()
		for _, e := range rmList {
			if _, ok := fm.eventTable[e.Filename]; ok {
				delete(fm.eventTable, e.Filename)
			}
		}
		fm.mu.Unlock()

		// wait for the poll period
		time.Sleep(fm.cfg.PollInterval)
	}
}

// Checks if the given file exists
func checkFileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}
