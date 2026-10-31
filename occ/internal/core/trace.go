package core

import (
	"log"
	"math"
	"slices"
)

// TraceEvent is an owned snapshot at a Process transition. Its slices may be
// retained or changed without changing transaction state. Zero timestamps
// mean unset. OtherID identifies a conflicting or purged transaction.
type TraceEvent struct {
	Kind     string   `json:"kind"`
	ID       string   `json:"id"`
	OtherID  string   `json:"otherID,omitempty"`
	Read     uint64   `json:"read"`
	Validate uint64   `json:"validate"`
	Write    uint64   `json:"write"`
	ReadSet  []string `json:"readSet"`
	WriteSet []string `json:"writeSet"`
	Error    string   `json:"error,omitempty"`
}

// Observer runs synchronously and may be called concurrently. It must return
// promptly and must not call back into a Manager. Panics are logged and contained.
// Tracing affects scheduling; use it for diagnostics, not benchmarks.
type Observer func(TraceEvent)

func trace(kind string, txn Transaction, other string, err error) {
	observe := txn.GetObserver()
	if observe == nil {
		return
	}
	stamp := func(t uint64) uint64 {
		if t >= math.MaxInt64 {
			return 0
		}
		return t
	}
	e := TraceEvent{Kind: kind, ID: txn.GetID(), OtherID: other,
		Read: stamp(txn.GetReadTime()), Validate: stamp(txn.GetValidateTime()), Write: stamp(txn.GetWriteTime()),
		ReadSet: slices.Sorted(txn.GetReadSet().All()), WriteSet: slices.Sorted(txn.GetWriteSet().All())}
	if err != nil {
		e.Error = err.Error()
	}
	defer func() {
		if p := recover(); p != nil {
			log.Printf("occ: observer panic: %v", p)
		}
	}()
	observe(e)
}
