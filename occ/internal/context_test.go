package occ

import (
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/mbb200291/go-occ-z/occ/internal/core"
)

type adapter struct {
	reads int
	err   error
}

func (a *adapter) Read(string) (any, error)      { a.reads++; return []int{1}, a.err }
func (*adapter) Write(string, any) error         { return nil }
func (*adapter) Serialize(any) ([]byte, error)   { return nil, nil }
func (*adapter) UnSerialize([]byte) (any, error) { return nil, nil }
func TestContextDependencies(t *testing.T) {
	a := &adapter{}
	c := NewOccContext(a)
	v, e := c.GetData("x")
	if e != nil {
		t.Fatal(e)
	}
	v.([]int)[0] = 2
	again, _ := c.GetData("x")
	if a.reads != 1 || again.([]int)[0] != 2 || !c.GetReadSet().Contains("x") {
		t.Fatal("read cache or dependency lost")
	}
	c.SetData("x", 3)
	got, _ := c.GetData("x")
	if got != 3 || !c.GetWriteSet().Contains("x") {
		t.Fatal("read-your-writes failed")
	}
	c.SetData("y", 4)
	_, _ = c.GetData("y")
	if c.GetReadSet().Contains("y") || !c.GetWriteSet().Contains("y") || a.reads != 1 {
		t.Fatal("private value incorrectly read from storage")
	}
	other := NewOccContext(a)
	v, _ = other.GetData("x")
	if v.([]int)[0] != 1 {
		t.Fatal("contexts shared state")
	}
}
func TestContextReadErrorNotCached(t *testing.T) {
	a := &adapter{err: os.ErrNotExist}
	c := NewOccContext(a)
	if _, e := c.GetData("x"); !errors.Is(e, core.ErrRead) || !errors.Is(e, os.ErrNotExist) {
		t.Fatal(e)
	}
	a.err = nil
	if _, e := c.GetData("x"); e != nil || a.reads != 2 {
		t.Fatalf("%v reads=%d", e, a.reads)
	}
}
func TestSetAndContainer(t *testing.T) {
	s := NewSet()
	s.Add([]string{"x", "y", "x"})
	if s.Size() != 2 || s.IsDisjoint(Set{"y": {}}) {
		t.Fatal(s)
	}
	s.Remove("y")
	if !s.IsDisjoint(Set{"y": {}}) {
		t.Fatal(s)
	}
	visits := 0
	for range s.All() {
		visits++
		break
	}
	if visits != 1 {
		t.Fatal(visits)
	}
	c := NewTransactionContainer(NewSkipMap(), func(t core.Transaction) uint64 { return t.GetValidateTime() })
	if _, ok := c.GetMin(); ok {
		t.Fatal("nonempty")
	}
	txns := make([]*OccTransaction, 3)
	for i := range txns {
		txns[i] = &OccTransaction{}
		txns[i].SetValidateTime(uint64(i + 1))
		c.Add(txns[i])
	}
	var keys []uint64
	c.IterTill(2, func(k uint64, _ core.Transaction) bool { keys = append(keys, k); return true })
	if !reflect.DeepEqual(keys, []uint64{1, 2}) {
		t.Fatal(keys)
	}
	visits = 0
	c.IterTill(3, func(uint64, core.Transaction) bool { visits++; return false })
	if visits != 1 {
		t.Fatal(visits)
	}
	c.Remove(txns[0])
	v, ok := c.GetMin()
	if !ok || v != txns[1] {
		t.Fatal(v, ok)
	}
}
