package pidmonitor

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"sort"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
)

func init() {
	//log.SetLevel(log.DebugLevel)
}

func pidListEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}

	sort.Ints(a)
	sort.Ints(b)

	for i, pid := range a {
		if b[i] != pid {
			return false
		}
	}
	return true
}

func eventListSort(a []PidEvent) {
	sort.SliceStable(a, func(i, j int) bool {
		return a[i].Pid < a[j].Pid
	})
}

func eventListEqual(a, b []PidEvent) bool {
	if len(a) != len(b) {
		return false
	}

	eventListSort(a)
	eventListSort(b)

	for i, event := range a {
		if b[i] != event {
			return false
		}
	}
	return true
}

func TestAddAndRemoveEvent(t *testing.T) {

	pidMonCfg := &Cfg{
		Poll: 500,
	}

	pidMon, err := New(pidMonCfg)
	if err != nil {
		t.Fatalf("New() failed: %s", err)
	}
	defer pidMon.Close()

	events := []PidEvent{
		{Pid: 1, Event: Exit},
		{Pid: 2, Event: Exit},
	}

	// verify Add
	if err := pidMon.AddEvent(events); err != nil {
		t.Errorf("AddEvent() failed: %s\n", err)
	}

	for _, e := range events {
		evect, found := pidMon.eventTable[e.Pid]
		if !found || evect != Exit {
			t.Errorf("AddEvent() failed: pid = %d, found = %v, evect = %x\n", e.Pid, found, evect)
		}
	}

	// verify Remove
	if err := pidMon.RemoveEvent(events); err != nil {
		t.Errorf("RemoveEvent() failed: %s\n", err)
	}

	for _, e := range events {
		_, found := pidMon.eventTable[e.Pid]
		if found {
			t.Errorf("RemoveEvent() failed: pid = %d, found = %v\n", e.Pid, found)
		}
	}

}

// spawns the given number of dummy processes; returns their pids.
func spawnDummyProcesses(num int) ([]int, error) {
	var err error

	pids := []int{}
	for i := 0; i < num; i++ {
		cmd := exec.Command("tail", "-f", "/dev/null")
		if err = cmd.Start(); err != nil {
			break
		}
		pids = append(pids, cmd.Process.Pid)
	}

	if err != nil {
		killDummyProcesses(pids)
		return nil, err
	}

	return pids, nil
}

// kills the processes with the given pids.
func killDummyProcesses(pids []int) error {
	for _, pid := range pids {
		proc, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("failed to find pid %d\n", pid)
		}
		// kill
		if err = proc.Kill(); err != nil {
			return fmt.Errorf("failed to kill pid %d\n", pid)
		}
		// reap
		_, err = proc.Wait()
		if err != nil {
			return fmt.Errorf("failed to reap pid %d\n", pid)
		}
	}
	return nil
}

func waitAndCheckEvent(t *testing.T, numProc int, pidMon *PidMon, want []PidEvent, resultCh chan error) {

	eventList := []PidEvent{}
	for {
		pidEvents := pidMon.WaitEvent()
		eventList = append(eventList, pidEvents...)
		if len(eventList) >= numProc {
			break
		}
	}

	if !eventListEqual(want, eventList) {
		resultCh <- fmt.Errorf("pidMon.Wait() failed: want %+v, got %+v\n", want, eventList)
		return
	}

	resultCh <- nil
}

func TestEventExit(t *testing.T) {

	numProc := 10

	pidMonCfg := &Cfg{
		Poll: 100,
	}

	pidMon, err := New(pidMonCfg)
	if err != nil {
		t.Fatalf("New() failed: %s", err)
	}
	defer pidMon.Close()

	pidList, err := spawnDummyProcesses(numProc)
	if err != nil {
		t.Fatalf("spawnDummyProcesses() failed: %s\n", err)
	}

	// create the event monitor list
	eventList := []PidEvent{}
	for _, pid := range pidList {
		eventList = append(eventList, PidEvent{uint32(pid), Exit, nil})
	}

	resultCh := make(chan error)

	go waitAndCheckEvent(t, numProc, pidMon, eventList, resultCh)

	if err := pidMon.AddEvent(eventList); err != nil {
		t.Fatalf("AddEvent() failed: %s\n", err)
	}

	// wait a bit such that the process kill occurs concurrently with the monitor checking
	// (otherwise the processes will likely be all killed before the monitor knows that it
	// has to check for them)
	time.Sleep(500 * time.Millisecond)

	// trigger process exit event
	if err := killDummyProcesses(pidList); err != nil {
		t.Fatalf("KillDummyProcesss() failed: %s\n", err)
	}

	// wait for event checker to be done
	if err := <-resultCh; err != nil {
		t.Fatalf("Event failed: %s", err)
	}
}

//
// The following functions are used by the TestEventExitConcurrent() test
//

// Spawns up to numProc processes at random intervals
func spawner(t *testing.T, numProc int, startCh chan bool, spawnedCh chan []int) {
	src := rand.NewSource(time.Now().UnixNano())
	random := rand.New(src)

	<-startCh

	log.Debugf("spawner: started ...\n")

	for i := 0; i < numProc; i++ {
		pidList, err := spawnDummyProcesses(1)
		if err != nil {
			t.Fatalf("spawnDummyProcesses() failed: %s\n", err)
		}

		spawnedCh <- pidList

		log.Debugf("spawner: spawned %v\n", pidList)

		delay := random.Intn(10)
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
}

// Kills spawned processes at random intervals
func killer(t *testing.T, numProc int, pidMon *PidMon, spawnedCh, killedCh chan []int) {
	src := rand.NewSource(time.Now().UnixNano())
	random := rand.New(src)

	killedList := []int{}

	for {
		// Listen to spawner
		spawnedList := <-spawnedCh

		// Tell pidMon to watch for exit event on the spawned processes
		eventList := []PidEvent{}
		for _, pid := range spawnedList {
			eventList = append(eventList, PidEvent{uint32(pid), Exit, nil})
		}
		if err := pidMon.AddEvent(eventList); err != nil {
			t.Fatalf("AddEvent() failed: %s\n", err)
		}

		// Kill the processes
		for _, pid := range spawnedList {
			if err := killDummyProcesses([]int{pid}); err != nil {
				t.Fatalf("KillDummyProcesss() failed: %s\n", err)
			}
			delay := random.Intn(10)
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}

		log.Debugf("killer: killed %v\n", spawnedList)

		killedList = append(killedList, spawnedList...)

		if len(killedList) >= numProc {
			break
		}
	}

	killedCh <- killedList
}

// Waits for the pid monitor events
func waiter(t *testing.T, numProc int, pidMon *PidMon, eventCh chan []int) {
	src := rand.NewSource(time.Now().UnixNano())
	random := rand.New(src)

	eventList := []int{}

	for {
		pidEvents := pidMon.WaitEvent()

		log.Debugf("waiter: events %v\n", pidEvents)

		for _, e := range pidEvents {
			if e.Event != Exit {
				t.Fatalf("pidMon reported non-exit event: pid = %d, event = %x\n", e.Pid, e.Event)
			}
			eventList = append(eventList, int(e.Pid))
		}

		if len(eventList) >= numProc {
			break
		}

		delay := random.Intn(10)
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	eventCh <- eventList
}

func TestEventExitConcurrent(t *testing.T) {

	numProc := 100

	pidMonCfg := &Cfg{
		Poll: 50,
	}

	pidMon, err := New(pidMonCfg)
	if err != nil {
		t.Fatalf("New() failed: %s", err)
	}
	defer pidMon.Close()

	// create spawner, killer, waiter threads
	startCh := make(chan bool)
	spawnedCh := make(chan []int, 100)
	killedCh := make(chan []int, 100)
	eventCh := make(chan []int, 100)

	go spawner(t, numProc, startCh, spawnedCh)
	go killer(t, numProc, pidMon, spawnedCh, killedCh)
	go waiter(t, numProc, pidMon, eventCh)

	// start spawning
	startCh <- true

	// wait for killer and checker to finish
	killedList := <-killedCh
	eventList := <-eventCh

	if !pidListEqual(eventList, killedList) {
		t.Fatalf("event list does not match kill list: events: %+v; killed: %+v\n", eventList, killedList)
	}
}
