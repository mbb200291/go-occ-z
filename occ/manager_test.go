package occ

import (
	"errors"
	"fmt"
	"github.com/mbb200291/go-occ-z/occ/internal/core"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type memoryAdapter struct {
	mu         sync.Mutex
	values     map[string]int
	writes     int
	failOnce   bool
	failAt     int
	alwaysFail bool
}

func (a *memoryAdapter) Read(k string) (any, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	v, ok := a.values[k]
	if !ok {
		return nil, os.ErrNotExist
	}
	return v, nil
}
func (a *memoryAdapter) Write(k string, v any) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.writes++
	a.values[k] = v.(int)
	if a.failOnce || a.alwaysFail || a.writes == a.failAt {
		a.failOnce = false
		return errors.New("injected partial write")
	}
	return nil
}
func (a *memoryAdapter) Serialize(v any) ([]byte, error)   { return []byte(strconv.Itoa(v.(int))), nil }
func (a *memoryAdapter) UnSerialize(b []byte) (any, error) { return strconv.Atoi(string(b)) }

type commandFunc func(Context) (any, error)

func (f commandFunc) Execute(c Context) (any, error) { return f(c) }
func readValue(k string) Command {
	return commandFunc(func(c Context) (any, error) { return c.GetData(k) })
}
func increment(k string) Command {
	return commandFunc(func(c Context) (any, error) {
		v, e := c.GetData(k)
		if e != nil {
			return nil, e
		}
		n := v.(int) + 1
		c.SetData(k, n)
		return n, nil
	})
}

func testManager(t *testing.T) (*occManager, *memoryAdapter) {
	t.Helper()
	t.Chdir(t.TempDir())
	a := &memoryAdapter{values: map[string]int{"x": 0, "y": 0}}
	mgr, err := NewManager(a, Config{Retry: &RetryOptions{MaxRetries: 0, BaseDelay: time.Nanosecond, MaxDelay: time.Nanosecond}})
	if err != nil {
		t.Fatal(err)
	}
	// Production locks live until process exit. Release only in this serial test
	// harness, after draining asynchronous cleanup and before restoring cwd.
	t.Cleanup(func() {
		deadline := time.Now().Add(5 * time.Second)
		for {
			entries, e := os.ReadDir(".occ")
			if e != nil {
				t.Error(e)
				break
			}
			pending := false
			for _, v := range entries {
				pending = pending || v.IsDir()
			}
			if !pending {
				break
			}
			if time.Now().After(deadline) {
				t.Error("cleanup did not finish")
				break
			}
			time.Sleep(time.Millisecond)
		}
		if e := mgr.lock.Close(); e != nil {
			t.Error(e)
		}
	})
	return mgr, a
}
func TestOutcomesAndReadYourWrites(t *testing.T) {
	m, a := testManager(t)
	out, e := m.Run([]Command{increment("x"), readValue("x"), increment("x")})
	if e != nil {
		t.Fatal(e)
	}
	if fmt.Sprint(out) != "[1 1 2]" {
		t.Fatal(out)
	}
	v, _ := a.Read("x")
	if v != 2 {
		t.Fatal(v)
	}
}
func TestReadOnlyDoesNotCommit(t *testing.T) {
	m, a := testManager(t)
	if _, e := m.Run([]Command{readValue("x")}); e != nil {
		t.Fatal(e)
	}
	if a.writes != 0 {
		t.Fatal(a.writes)
	}
}
func TestCommandFailureDoesNotPublishOrRetry(t *testing.T) {
	m, a := testManager(t)
	sentinel := errors.New("command failed")
	calls := 0
	out, e := m.Run([]Command{commandFunc(func(c Context) (any, error) { calls++; c.SetData("x", 99); return 99, sentinel })})
	if out != nil || !errors.Is(e, ErrExecute) || !errors.Is(e, sentinel) || calls != 1 {
		t.Fatalf("out=%v err=%v calls=%d", out, e, calls)
	}
	v, _ := a.Read("x")
	if v != 0 {
		t.Fatal(v)
	}
}
func TestWrappedConflictIsNotRetried(t *testing.T) {
	m, _ := testManager(t)
	calls := 0
	_, e := m.Run([]Command{commandFunc(func(Context) (any, error) { calls++; return nil, ErrConflict })})
	if calls != 1 || !errors.Is(e, ErrExecute) {
		t.Fatalf("calls=%d err=%v", calls, e)
	}
}
func TestReadErrorIsPreserved(t *testing.T) {
	m, _ := testManager(t)
	_, e := m.Run([]Command{readValue("missing")})
	if !errors.Is(e, ErrRead) || !errors.Is(e, os.ErrNotExist) || !errors.Is(e, ErrExecute) {
		t.Fatal(e)
	}
}
func TestPartialWriteRollsBack(t *testing.T) {
	m, a := testManager(t)
	a.failOnce = true
	out, e := m.Run([]Command{increment("x")})
	if out != nil || !errors.Is(e, ErrCommit) {
		t.Fatalf("out=%v err=%v", out, e)
	}
	v, _ := a.Read("x")
	if v != 0 || a.writes != 2 {
		t.Fatalf("value=%v writes=%d", v, a.writes)
	}
}
func TestConflictReexecutesWithFreshContext(t *testing.T) {
	m, a := testManager(t)
	read := make(chan struct{})
	release := make(chan struct{})
	var attempts atomic.Int32
	cmd := commandFunc(func(c Context) (any, error) {
		v, e := c.GetData("x")
		if e != nil {
			return nil, e
		}
		if attempts.Add(1) == 1 {
			close(read)
			<-release
		}
		c.SetData("x", v.(int)+1)
		return v.(int) + 1, nil
	})
	type result struct {
		o []any
		e error
	}
	done := make(chan result, 1)
	go func() { o, e := m.Run([]Command{cmd}); done <- result{o, e} }()
	<-read
	_, err := m.Run([]Command{increment("x")})
	close(release)
	r := <-done
	if err != nil || r.e != nil {
		t.Fatalf("%v %v", err, r.e)
	}
	v, _ := a.Read("x")
	if attempts.Load() != 2 || v != 2 || len(r.o) != 1 || r.o[0] != 2 {
		t.Fatalf("attempts=%d value=%v outcomes=%v", attempts.Load(), v, r.o)
	}
}
func TestConcurrentIncrements(t *testing.T) {
	m, a := testManager(t)
	const count = 16
	start := make(chan struct{})
	errs := make(chan error, count)
	for range count {
		go func() { <-start; _, e := m.Run([]Command{increment("x")}); errs <- e }()
	}
	close(start)
	for range count {
		if e := <-errs; e != nil {
			t.Fatal(e)
		}
	}
	v, _ := a.Read("x")
	if v != count {
		t.Fatal(v)
	}
}
func TestExclusiveRunBlocksOtherCommands(t *testing.T) {
	m, _ := testManager(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, e := m.RunExclusive([]Command{commandFunc(func(c Context) (any, error) { close(entered); <-release; return nil, nil })})
		done <- e
	}()
	<-entered
	if m.runMu.TryRLock() {
		m.runMu.RUnlock()
		close(release)
		<-done
		t.Fatal("exclusive transaction did not hold write lock")
	}
	close(release)
	if e := <-done; e != nil {
		t.Fatal(e)
	}
}
func TestNilCommandRejected(t *testing.T) {
	m, _ := testManager(t)
	_, e := m.Run([]Command{nil})
	if !errors.Is(e, ErrIniTxn) {
		t.Fatalf("expected ErrIniTxn, got %v", e)
	}
}
func TestEmptyTransactionRejected(t *testing.T) {
	m, _ := testManager(t)
	_, e := m.Run(nil)
	if !errors.Is(e, ErrIniTxn) {
		t.Fatal(e)
	}
}
func TestManagerValidation(t *testing.T) {
	if _, e := NewManager(nil); !errors.Is(e, ErrIniManager) {
		t.Fatal(e)
	}
	for _, c := range []Config{{Retry: &RetryOptions{}}, {Retry: &RetryOptions{MaxRetries: -1, BaseDelay: 1, MaxDelay: 1}}, {Retry: &RetryOptions{BaseDelay: 2, MaxDelay: 1}}} {
		if _, e := resolveConfig([]Config{c}); !errors.Is(e, ErrIniManager) {
			t.Fatal(e)
		}
	}
	if _, e := resolveConfig([]Config{{}, {}}); !errors.Is(e, ErrIniManager) {
		t.Fatal(e)
	}
	o, e := resolveConfig(nil)
	if e != nil || o.MaxRetries != 32 || o.Jitter {
		t.Fatalf("%+v %v", o, e)
	}
}
func TestDirectoryLockAcrossProcesses(t *testing.T) {
	if os.Getenv("OCC_TEST_LOCK_CHILD") == "1" {
		_, e := NewManager(&memoryAdapter{})
		if !errors.Is(e, ErrDirectoryInUse) {
			t.Fatalf("expected directory lock error, got %v", e)
		}
		return
	}
	testManager(t)
	if _, e := NewManager(&memoryAdapter{}); !errors.Is(e, ErrDirectoryInUse) {
		t.Fatal(e)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestDirectoryLockAcrossProcesses$", "-test.timeout=10s")
	cmd.Env = append(os.Environ(), "OCC_TEST_LOCK_CHILD=1")
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("child: %v %s", e, b)
	}
}

func TestMultiTargetRollback(t *testing.T) {
	m, a := testManager(t)
	a.failAt = 2
	out, e := m.Run([]Command{increment("x"), increment("y")})
	if out != nil || !errors.Is(e, ErrCommit) {
		t.Fatalf("%v %v", out, e)
	}
	for _, k := range []string{"x", "y"} {
		v, _ := a.Read(k)
		if v != 0 {
			t.Fatalf("%s=%v", k, v)
		}
	}
	if a.writes != 4 {
		t.Fatal(a.writes)
	}
}
func TestRollbackFailurePanicsAndPreservesRecovery(t *testing.T) {
	m, a := testManager(t)
	a.alwaysFail = true
	var caught any
	func() { defer func() { caught = recover() }(); _, _ = m.Run([]Command{increment("x")}) }()
	e, ok := caught.(error)
	if !ok || !errors.Is(e, ErrRollback) {
		t.Fatalf("panic=%v", caught)
	}
	a.alwaysFail = false
	if _, e := core.Recovery(a); e != nil {
		t.Fatal(e)
	}
	v, _ := a.Read("x")
	if v != 0 {
		t.Fatal(v)
	}
}
func TestStartupRecoveryFailureReleasesLock(t *testing.T) {
	t.Chdir(t.TempDir())
	if e := os.MkdirAll(".occ/bad", 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(".occ/bad/journal", []byte("BROKEN\n"), 0600); e != nil {
		t.Fatal(e)
	}
	a := &memoryAdapter{}
	if _, e := NewManager(a); !errors.Is(e, ErrIniManager) {
		t.Fatal(e)
	}
	lock, e := acquireLock()
	if e != nil {
		t.Fatal("failed startup retained lock:", e)
	}
	if e := lock.Close(); e != nil {
		t.Fatal(e)
	}
}

func TestDisjointTransactionsDoNotRetry(t *testing.T) {
	m, _ := testManager(t)
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	done := make(chan error, 2)
	var calls atomic.Int32
	for _, key := range []string{"x", "y"} {
		go func(k string) {
			_, e := m.Run([]Command{commandFunc(func(c Context) (any, error) {
				v, e := c.GetData(k)
				if e != nil {
					return nil, e
				}
				calls.Add(1)
				ready <- struct{}{}
				<-release
				c.SetData(k, v.(int)+1)
				return nil, nil
			})})
			done <- e
		}(key)
	}
	<-ready
	<-ready
	close(release)
	for range 2 {
		if e := <-done; e != nil {
			t.Fatal(e)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("disjoint transactions retried: %d", calls.Load())
	}
}
