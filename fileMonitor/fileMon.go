//
// Copyright 2023 Nestybox Inc.
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

// The fileMonitor notifies the caller about file removal events.
// It uses a simple polling algorithm.

package fileMonitor

import (
	"fmt"
	"sync"
	"time"
)

type Cfg struct {
	EventBufSize int
	PollInterval time.Duration // in milliseconds
}

// polling config limits
const (
	PollMin = 1 * time.Millisecond
	PollMax = 10000 * time.Millisecond
)

type Event struct {
	Filename string
	Err      error
}

type FileMon struct {
	mu         sync.Mutex
	cfg        Cfg
	eventTable map[string]bool
	cmdCh      chan cmd
	eventCh    chan []Event // receives events from monitor thread
}

func New(cfg *Cfg) (*FileMon, error) {
	if err := validateCfg(cfg); err != nil {
		return nil, err
	}

	fm := &FileMon{
		cfg:        *cfg,
		eventTable: make(map[string]bool),
		cmdCh:      make(chan cmd),
		eventCh:    make(chan []Event, cfg.EventBufSize),
	}

	go fileMon(fm)

	return fm, nil
}

func (fm *FileMon) Add(file string) {
	fm.mu.Lock()
	fm.eventTable[file] = true
	fm.mu.Unlock()
}

func (fm *FileMon) Remove(file string) {
	fm.mu.Lock()
	if _, ok := fm.eventTable[file]; ok {
		delete(fm.eventTable, file)
	}
	fm.mu.Unlock()
}

func (fm *FileMon) Events() <-chan []Event {
	return fm.eventCh
}

func (fm *FileMon) Close() {
	fm.cmdCh <- stop
}

func validateCfg(cfg *Cfg) error {
	if cfg.PollInterval < PollMin || cfg.PollInterval > PollMax {
		return fmt.Errorf("invalid config: poll interval must be in range [%d, %d]; found %d", PollMin, PollMax, cfg.PollInterval)
	}
	return nil
}
