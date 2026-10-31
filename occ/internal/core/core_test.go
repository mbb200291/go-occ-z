package core

import (
	"errors"
	"fmt"
	"iter"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testSet map[string]bool

func (s testSet) Contains(k string) bool { return s[k] }
func (s testSet) Size() int              { return len(s) }
func (s testSet) All() iter.Seq[string] {
	return func(yield func(string) bool) {
		for k := range s {
			if !yield(k) {
				return
			}
		}
	}
}
func (s testSet) IsDisjoint(o Set) bool {
	for k := range s {
		if o.Contains(k) {
			return false
		}
	}
	return true
}

type testTxn struct {
	id            string
	r, v, w       uint64
	reads, writes testSet
	data          map[string]any
}

func (t *testTxn) GetID() string             { return t.id }
func (t *testTxn) GetReadTime() uint64       { return t.r }
func (t *testTxn) GetValidateTime() uint64   { return t.v }
func (t *testTxn) GetWriteTime() uint64      { return t.w }
func (t *testTxn) SetReadTime(v uint64)      { t.r = v }
func (t *testTxn) SetValidateTime(v uint64)  { t.v = v }
func (t *testTxn) SetWriteTime(v uint64)     { t.w = v }
func (t *testTxn) GetReadSet() Set           { return t.reads }
func (t *testTxn) GetWriteSet() Set          { return t.writes }
func (t *testTxn) GetWriteData(k string) any { return t.data[k] }

func TestValidationRules(t *testing.T) {
	for _, tc := range []struct {
		name        string
		finish      uint64
		read, write string
		want        bool
	}{
		{"completed before read", 9, "x", "x", true},
		{"overlapping read", 15, "x", "y", false},
		{"completed blind write", 15, "y", "x", true},
		{"inflight write/write", math.MaxInt64, "y", "x", false},
		{"inflight disjoint", math.MaxInt64, "y", "z", true},
		{"equal read timestamp is overlap", 10, "x", "y", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prev := &testTxn{w: tc.finish, writes: testSet{"x": true}}
			cur := &testTxn{r: 10, w: math.MaxInt64, reads: testSet{tc.read: true}, writes: testSet{tc.write: true}}
			if got := validatePair(prev, cur); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
func TestRetryOnlyBareConflict(t *testing.T) {
	sentinel := errors.New("storage")
	for _, tc := range []struct {
		name  string
		err   error
		calls int
	}{
		{"success", nil, 1}, {"conflict", ErrConflict, 3}, {"wrapped conflict", fmt.Errorf("command: %w", ErrConflict), 1}, {"storage", sentinel, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := 0
			err := RetryProcess(RetryOptions{MaxRetries: 2, BaseDelay: time.Nanosecond, MaxDelay: time.Nanosecond}, func() error { n++; return tc.err })
			if n != tc.calls || err != tc.err {
				t.Fatalf("calls=%d err=%v", n, err)
			}
		})
	}
}
func TestRetryJitterBounds(t *testing.T) {
	for _, d := range []time.Duration{1, 3, time.Millisecond, time.Second} {
		if got := retryWait(d, false); got != d {
			t.Fatal(got)
		}
		for range 100 {
			got := retryWait(d, true)
			if got < d-d/2 || got > d {
				t.Fatalf("delay=%v wait=%v", d, got)
			}
		}
	}
}

type testAdapter struct {
	values    map[string]string
	failWrite bool
	writes    int
}

func (a *testAdapter) Read(k string) (any, error) {
	v, ok := a.values[k]
	if !ok {
		return nil, os.ErrNotExist
	}
	return v, nil
}
func (a *testAdapter) Write(k string, v any) error {
	a.writes++
	if a.failWrite {
		return errors.New("write failed")
	}
	a.values[k] = v.(string)
	return nil
}
func (a *testAdapter) Serialize(v any) ([]byte, error)   { return []byte(v.(string)), nil }
func (a *testAdapter) UnSerialize(b []byte) (any, error) { return string(b), nil }
func prepareJournal(t *testing.T, id, target, tail string) {
	t.Helper()
	if err := saveBackup(id, target, []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := appendLog(id, "WRITE", target); err != nil {
		t.Fatal(err)
	}
	if tail != "" {
		f, e := os.OpenFile(journalPath(id), os.O_APPEND|os.O_WRONLY, 0600)
		if e != nil {
			t.Fatal(e)
		}
		_, e = f.WriteString(tail)
		if e != nil {
			t.Fatal(e)
		}
		if e = f.Close(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestRecoveryJournalStates(t *testing.T) {
	for _, tc := range []struct {
		name, tail string
		committed  bool
		want       string
	}{{"unfinished", "", false, "old"}, {"partial revert", "REV", false, "old"}, {"partial commit", "COMM", false, "old"}, {"committed", "COMMIT\n", true, "new"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			target := "path with spaces/電影\n"
			prepareJournal(t, "txn", target, tc.tail)
			a := &testAdapter{values: map[string]string{target: "new"}}
			ids, err := Recovery(a)
			if err != nil {
				t.Fatal(err)
			}
			if (len(ids) == 1) != tc.committed || a.values[target] != tc.want {
				t.Fatalf("ids=%v values=%v", ids, a.values)
			}
			if _, e := os.Stat(transactionDir("txn")); !os.IsNotExist(e) {
				t.Fatalf("not cleaned: %v", e)
			}
			if _, err := Recovery(a); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestRecoveryCanResumeAfterFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	prepareJournal(t, "txn", "x", "REV")
	a := &testAdapter{values: map[string]string{"x": "new"}, failWrite: true}
	if _, err := Recovery(a); err == nil {
		t.Fatal("expected error")
	}
	b, err := os.ReadFile(journalPath("txn"))
	if err != nil || !strings.HasSuffix(string(b), "\n") {
		t.Fatalf("tail not truncated: %q %v", b, err)
	}
	a.failWrite = false
	if _, err := Recovery(a); err != nil {
		t.Fatal(err)
	}
	if a.values["x"] != "old" {
		t.Fatal(a.values)
	}
}
func TestRecoveryRejectsCorruptCompleteRecord(t *testing.T) {
	t.Chdir(t.TempDir())
	prepareJournal(t, "txn", "x", "BROKEN\n")
	before, _ := os.ReadFile(journalPath("txn"))
	a := &testAdapter{values: map[string]string{"x": "new"}}
	if _, err := Recovery(a); err == nil {
		t.Fatal("expected error")
	}
	after, _ := os.ReadFile(journalPath("txn"))
	if string(before) != string(after) || a.writes != 0 {
		t.Fatal("corrupt journal mutated storage")
	}
}
func TestRecoveryWithoutJournal(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join(transactionDir("txn"), "backup"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := Recovery(&testAdapter{}); err != nil {
		t.Fatal(err)
	}
	if _, e := os.Stat(transactionDir("txn")); !os.IsNotExist(e) {
		t.Fatal(e)
	}
}
func TestRecoverySkipsRevertedTarget(t *testing.T) {
	t.Chdir(t.TempDir())
	prepareJournal(t, "txn", "x", "")
	if err := appendLog("txn", "REVERT", "x"); err != nil {
		t.Fatal(err)
	}
	if err := discardBackup("txn", "x"); err != nil {
		t.Fatal(err)
	}
	a := &testAdapter{values: map[string]string{"x": "old"}}
	if _, err := Recovery(a); err != nil {
		t.Fatal(err)
	}
	if a.writes != 0 {
		t.Fatal("replayed completed rollback")
	}
}

func TestRecoveryAfterProcessExit(t *testing.T) {
	if os.Getenv("OCC_TEST_CRASH") == "1" {
		prepareJournal(t, "crashed", "x", "REV")
		os.Exit(17)
	}
	t.Chdir(t.TempDir())
	cmd := exec.Command(os.Args[0], "-test.run=^TestRecoveryAfterProcessExit$")
	cmd.Env = append(os.Environ(), "OCC_TEST_CRASH=1")
	b, e := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(e, &exit) || exit.ExitCode() != 17 {
		t.Fatalf("child %v %s", e, b)
	}
	a := &testAdapter{values: map[string]string{"x": "new"}}
	if _, e := Recovery(a); e != nil {
		t.Fatal(e)
	}
	if a.values["x"] != "old" {
		t.Fatal(a.values)
	}
}

func (t *testTxn) GetObserver() Observer { return nil }
