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

package fileMonitor

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/nestybox/sysbox-libs/utils"
	log "github.com/sirupsen/logrus"
)

func init() {
	//log.SetLevel(log.DebugLevel)
}

func TestOneRemovalPerInterval(t *testing.T) {

	numFiles := 5

	// create a bunch of temp files
	tmpFiles := []string{}
	for i := 0; i < numFiles; i++ {
		file, err := ioutil.TempFile("", "fileMonTest")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(file.Name())
		tmpFiles = append(tmpFiles, file.Name())
		t.Logf("Created file %s", file.Name())
	}

	// create a new file mon
	pollInterval := 100 * time.Millisecond
	cfg := Cfg{
		EventBufSize: 10,
		PollInterval: pollInterval,
	}
	fm, err := New(&cfg)
	if err != nil {
		t.Fatal(err)
	}

	// watch files
	for _, file := range tmpFiles {
		fm.Add(file)
	}
	fileEvents := fm.Events()

	// remove one file at a time (one per poll interval)
	for _, file := range tmpFiles {
		if err := os.Remove(file); err != nil {
			t.Fatal(err)
		}
		t.Logf("Removed file %s", file)
		time.Sleep(pollInterval)
		events := <-fileEvents
		if len(events) != 1 {
			t.Fatalf("incorrect events list size: want 1, got %d (%+v)", len(events), events)
		}
		e := events[0]
		if e.Filename != file {
			t.Fatalf("incorrect event file name: want %s, got %s", file, e.Filename)
		}
		if e.Err != nil {
			t.Fatalf("event has error: %s", e.Err)
		}
		t.Logf("OK: got event for file %s", e.Filename)
	}

	fm.Close()
	log.Debugf("Done.")
}

func TestMultiRemovalPerInterval(t *testing.T) {

	numFiles := 5

	// create a bunch of temp files
	tmpFiles := []string{}
	for i := 0; i < numFiles; i++ {
		file, err := ioutil.TempFile("", "fileMonTest")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(file.Name())
		tmpFiles = append(tmpFiles, file.Name())
		t.Logf("Created file %s", file.Name())
	}

	// create a new file mon
	pollInterval := 100 * time.Millisecond
	cfg := Cfg{
		EventBufSize: 10,
		PollInterval: pollInterval,
	}
	fm, err := New(&cfg)
	if err != nil {
		t.Fatal(err)
	}

	// watch files
	for _, file := range tmpFiles {
		fm.Add(file)
	}
	fileEvents := fm.Events()

	// remove all files in a single poll interval
	time.Sleep(pollInterval)

	for _, file := range tmpFiles {
		if err := os.Remove(file); err != nil {
			t.Fatal(err)
		}
		t.Logf("Removed file %s", file)
	}

	// verify we got all events
	time.Sleep(2 * pollInterval)

	events := []Event{}
	for {
		events = append(events, <-fileEvents...)
		numEvents := len(events)
		if numEvents == numFiles {
			break
		} else if numEvents > numFiles {
			t.Fatalf("got more file removal events than files (want %d, got %d)", numFiles, numEvents)
		}
	}

	for _, e := range events {
		if !utils.StringSliceContains(tmpFiles, e.Filename) {
			t.Fatalf("event %+v does not match a removed file", e)
		}
		if e.Err != nil {
			t.Fatalf("event has error: %s", e.Err)
		}
		t.Logf("OK: got event for file %s", e.Filename)
	}

	fm.Close()
}

func TestSymlinkedFileRemoval(t *testing.T) {

	numFiles := 5
	tmpFiles := []string{}
	symlinks := []string{}

	// create a bunch of temp files with symlinks to them
	for i := 0; i < numFiles; i++ {
		file, err := ioutil.TempFile("", "fileMonTest")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(file.Name())

		// create symlink to file
		link := fmt.Sprintf("symlink%d", i)
		if err := os.Symlink(file.Name(), link); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(link)

		tmpFiles = append(tmpFiles, file.Name())
		symlinks = append(symlinks, link)
		t.Logf("Created file %s and symlink %s", file.Name(), link)
	}

	// create a new file mon
	pollInterval := 100 * time.Millisecond
	cfg := Cfg{
		EventBufSize: 10,
		PollInterval: pollInterval,
	}
	fm, err := New(&cfg)
	if err != nil {
		t.Fatal(err)
	}

	// watch the symlinks
	for _, file := range symlinks {
		fm.Add(file)
	}
	fileEvents := fm.Events()

	// remove one file at a time, verify we get the event
	for i := 0; i < numFiles; i++ {
		file := tmpFiles[i]
		link := symlinks[i]

		if err := os.Remove(file); err != nil {
			t.Fatal(err)
		}
		t.Logf("Removed file %s", file)
		time.Sleep(pollInterval)
		events := <-fileEvents
		if len(events) != 1 {
			t.Fatalf("incorrect events list size: want 1, got %d (%+v)", len(events), events)
		}
		e := events[0]
		if e.Filename != link {
			t.Fatalf("incorrect event file name: want %s, got %s", link, e.Filename)
		}
		if e.Err != nil {
			t.Fatalf("event has error: %s", e.Err)
		}
		t.Logf("OK: got event for file %s", e.Filename)
	}

	fm.Close()
	log.Debugf("Done.")
}

func TestEventRemoval(t *testing.T) {

	numFiles := 5

	// create a bunch of temp files
	tmpFiles := []string{}
	for i := 0; i < numFiles; i++ {
		file, err := ioutil.TempFile("", "fileMonTest")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(file.Name())
		tmpFiles = append(tmpFiles, file.Name())
		t.Logf("Created file %s", file.Name())
	}

	// create a new file mon
	pollInterval := 100 * time.Millisecond
	cfg := Cfg{
		EventBufSize: 10,
		PollInterval: pollInterval,
	}
	fm, err := New(&cfg)
	if err != nil {
		t.Fatal(err)
	}

	// watch files
	for _, file := range tmpFiles {
		fm.Add(file)
	}
	fileEvents := fm.Events()

	// remove event for last file
	last := len(tmpFiles) - 1
	lastFile := tmpFiles[last]
	fm.Remove(lastFile)

	// Remove all files
	for _, file := range tmpFiles {
		if err := os.Remove(file); err != nil {
			t.Fatal(err)
		}
		t.Logf("Removed file %s", file)
	}

	// Verify notification was received for all files, except the last file
	time.Sleep(2 * pollInterval)

	events := []Event{}
	for {
		events = append(events, <-fileEvents...)
		numEvents := len(events)
		if numEvents == numFiles-1 {
			break
		} else if numEvents > numFiles-1 {
			t.Fatalf("got more file removal events than files (want %d, got %d)", numFiles-1, numEvents)
		}
	}

	for _, e := range events {
		if e.Filename == lastFile {
			t.Fatalf("event %+v should not have been received", e)
		}
		if !utils.StringSliceContains(tmpFiles, e.Filename) {
			t.Fatalf("event %+v does not match a removed file", e)
		}
		if e.Err != nil {
			t.Fatalf("event has error: %s", e.Err)
		}
		t.Logf("OK: got event for file %s", e.Filename)
	}
}

func TestEventOnNonExistentFile(t *testing.T) {

	// create a new file mon
	pollInterval := 100 * time.Millisecond
	cfg := Cfg{
		EventBufSize: 10,
		PollInterval: pollInterval,
	}
	fm, err := New(&cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Watch a non-existent file
	file := "/tmp/__doesnotexist__"
	fm.Add(file)

	// Should return event indicating file does not exist
	events := <-fm.Events()

	if len(events) != 1 {
		t.Fatalf("incorrect number of events; want 1, got %d (%+v)", len(events), events)
	}

	e := events[0]

	if e.Err != nil {
		t.Fatalf("event has error: %v", err)
	}

	if e.Filename != file {
		t.Fatalf("incorrect event filename: want %s, got %s", file, e.Filename)
	}

	fm.Close()
}
