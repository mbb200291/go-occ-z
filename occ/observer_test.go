package occ

import (
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mbb200291/go-occ-z/occ/internal/core"
)

func kinds(events []TraceEvent) []string {
	var out []string
	for _, e := range events {
		out = append(out, e.Kind)
	}
	return out
}
func TestObservedSuccess(t *testing.T) {
	var events []TraceEvent
	m, _ := testManager(t, func(e TraceEvent) { events = append(events, e) })
	out, err := m.Run([]Command{increment("x")})
	if err != nil || out[0] != 1 {
		t.Fatal(out, err)
	}
	want := []string{"read", "validate", "commit", "write", "end"}
	if !reflect.DeepEqual(kinds(events), want) {
		t.Fatal(kinds(events))
	}
	first, last := events[0], events[len(events)-1]
	if first.Read == 0 || first.Validate != 0 || first.Write != 0 || last.Read >= last.Validate || last.Validate >= last.Write || last.Error != "" {
		t.Fatalf("first=%+v last=%+v", first, last)
	}
	for _, e := range events {
		if e.ID != first.ID {
			t.Fatal("attempt ID changed")
		}
	}
	if !reflect.DeepEqual(last.ReadSet, []string{"x"}) || !reflect.DeepEqual(last.WriteSet, []string{"x"}) {
		t.Fatal(last)
	}
}
func TestObservedReadOnly(t *testing.T) {
	var events []TraceEvent
	m, _ := testManager(t, func(e TraceEvent) { events = append(events, e) })
	if _, err := m.Run([]Command{readValue("x")}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(kinds(events), []string{"read", "validate", "write", "end"}) {
		t.Fatal(kinds(events))
	}
	if _, err := m.Run([]Command{increment("x")}); err != nil {
		t.Fatal(err)
	}
}
func TestObservedFailure(t *testing.T) {
	sentinel := errors.New("command failed")
	var events []TraceEvent
	m, _ := testManager(t, func(e TraceEvent) { events = append(events, e) })
	out, err := m.Run([]Command{commandFunc(func(Context) (any, error) { return nil, sentinel })})
	if out != nil || !errors.Is(err, sentinel) {
		t.Fatal(out, err)
	}
	if !reflect.DeepEqual(kinds(events), []string{"read", "end"}) || events[1].Error == "" || events[1].Write != 0 {
		t.Fatal(events)
	}
}
func TestObserverCannotMutateDependencies(t *testing.T) {
	notified := false
	m, a := testManager(t, func(e TraceEvent) {
		notified = true
		if len(e.WriteSet) > 0 {
			e.WriteSet[0] = "y"
		}
		if len(e.ReadSet) > 0 {
			e.ReadSet[0] = "y"
		}
	})
	_, err := m.RunExclusive([]Command{increment("x")})
	if err != nil {
		t.Fatal(err)
	}
	x, _ := a.Read("x")
	y, _ := a.Read("y")
	if !notified || x != 1 || y != 0 {
		t.Fatal(x, y)
	}
}
func TestObserverPanicDoesNotAbortTransaction(t *testing.T) {
	m, a := testManager(t, func(TraceEvent) { panic("bad observer") })
	if _, err := m.Run([]Command{increment("x")}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Run([]Command{increment("x")}); err != nil {
		t.Fatal(err)
	}
	v, _ := a.Read("x")
	if v != 2 {
		t.Fatal(v)
	}
}
func TestObservedConflictAndExclusiveFallback(t *testing.T) {
	ready := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	var events []TraceEvent
	var mu sync.Mutex
	m, _ := testManager(t, func(e TraceEvent) { mu.Lock(); defer mu.Unlock(); events = append(events, e) })
	done := make(chan error, 1)
	go func() {
		_, err := m.Run([]Command{commandFunc(func(c Context) (any, error) {
			v, e := c.GetData("x")
			if e != nil {
				return nil, e
			}
			if calls.Add(1) == 1 {
				close(ready)
				<-release
			}
			c.SetData("x", v.(int)+1)
			return v.(int) + 1, nil
		})})
		done <- err
	}()
	<-ready
	_, err := m.Run([]Command{increment("x")})
	close(release)
	e := <-done
	if err != nil || e != nil {
		t.Fatal(err, e)
	}
	ids := map[string]bool{}
	blocked, conflict := false, false
	for _, e := range events {
		if e.Kind == "read" {
			ids[e.ID] = true
		}
		if e.Kind == "blocked" {
			blocked = e.OtherID != ""
		}
		if e.Kind == "conflict" {
			conflict = e.Error != ""
		}
	}
	if len(ids) != 3 || !blocked || !conflict || events[len(events)-1].Error != "" {
		t.Fatal(events)
	}
}
func TestObservedRollback(t *testing.T) {
	var events []TraceEvent
	m, a := testManager(t, func(e TraceEvent) { events = append(events, e) })
	a.failOnce = true
	_, err := m.Run([]Command{increment("x")})
	if !errors.Is(err, ErrCommit) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(kinds(events), []string{"read", "validate", "commit", "rollback", "end"}) {
		t.Fatal(kinds(events))
	}
	last := events[len(events)-1]
	if last.Write == 0 || last.Error == "" {
		t.Fatal(last)
	}
}
func TestConcurrentObservers(t *testing.T) {
	var mu sync.Mutex
	ends := 0
	observe := func(e TraceEvent) {
		mu.Lock()
		defer mu.Unlock()
		if e.Kind == "end" {
			ends++
		}
	}
	m, _ := testManager(t, observe)
	done := make(chan error, 8)
	for range 8 {
		go func() { _, err := m.Run([]Command{readValue("x")}); done <- err }()
	}
	for range 8 {
		if e := <-done; e != nil {
			t.Fatal(e)
		}
	}
	if ends != 8 {
		t.Fatal(ends)
	}
}

func TestObserverDoesNotHoldTimestampLocks(t *testing.T) {
	for _, stage := range []string{"read", "validate"} {
		t.Run(stage, func(t *testing.T) {
			entered := make(chan struct{})
			release := make(chan struct{})
			done := make(chan error, 1)
			var first atomic.Bool
			m, _ := testManager(t, func(e TraceEvent) {
				if e.Kind == stage && first.CompareAndSwap(false, true) {
					close(entered)
					<-release
				}
			})
			go func() { _, err := m.Run([]Command{readValue("x")}); done <- err }()
			<-entered
			other := make(chan error, 1)
			go func() { _, err := m.Run([]Command{readValue("y")}); other <- err }()
			var err error
			blocked := false
			select {
			case err = <-other:
			case <-time.After(2 * time.Second):
				blocked = true
			}
			close(release)
			if e := <-done; e != nil {
				t.Fatal(e)
			}
			if blocked {
				<-other
				t.Fatal("observer blocked another transaction's timestamp registration")
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestObservedRollbackPanicStillPropagates(t *testing.T) {
	var last TraceEvent
	m, a := testManager(t, func(e TraceEvent) { last = e })
	a.alwaysFail = true
	var caught any
	func() {
		defer func() { caught = recover() }()
		_, _ = m.Run([]Command{increment("x")})
	}()
	a.alwaysFail = false
	if _, err := core.Recovery(a); err != nil {
		t.Fatal(err)
	}
	err, ok := caught.(error)
	if !ok || !errors.Is(err, ErrRollback) {
		t.Fatal(caught)
	}
	if last.Kind != "end" || last.Error == "" || last.Write != 0 {
		t.Fatal(last)
	}
}
func TestObservedPurge(t *testing.T) {
	var previous string
	found := false
	m, _ := testManager(t, func(e TraceEvent) {
		if previous == "" {
			previous = e.ID
		}
		if e.Kind == "purge" && e.OtherID == previous {
			found = true
		}
	})
	if _, err := m.Run([]Command{increment("x")}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Run([]Command{readValue("y")}); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("completed history removal was not reported")
	}
}
