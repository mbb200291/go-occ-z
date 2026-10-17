package core

import (
	"errors"
	"fmt"
	"iter"
	"math"
	"sync"
	"sync/atomic"
)

// prevTxn should use validate time as key
// ongoingTxn should use read time as key
func Process(txn Transaction, ctx Context, adpt TargetAdapter, prevTxns, ongoingTxns TransactionContainer) error {
	// ensure write time is max
	txn.SetWriteTime(uint64(math.MaxInt64))

	// read phase -- load read set to private zone
	muR.Lock() // [+] lock
	txn.SetReadTime(NextTimestamp())

	// register txn to ongoing T and all T
	ongoingTxns.Add(txn) // ordered by readtime
	muR.Unlock()         // [-] unlock

	// unregister txn from ongoing T after txn complete
	defer ongoingTxns.Remove(txn)

	err := Read(ctx, adpt, txn.GetTargetSet())
	if err != nil {
		err = fmt.Errorf("%w : %w", ErrRead, err)
		return err
	}

	// execute transaction optimicaly
	err = txn.Execute()
	if err != nil {
		err = fmt.Errorf("%w : %w", ErrExecute, err)
		return err
	}

	// set validate
	muV.Lock() // [+] lock
	txn.SetValidateTime(NextTimestamp())

	// add txn to prevTxn
	prevTxns.Add(txn)
	muV.Unlock() // [-] unlock

	// prepare purge write time till to
	minTxnReadTime := uint64(math.MaxInt64)
	if minTxn, exist := ongoingTxns.GetMin(); exist {
		minTxnReadTime = minTxn.GetReadTime()
	}

	// run validate
	if outcome := Validate(txn, prevTxns, minTxnReadTime); !outcome {
		prevTxns.Remove(txn)
		return errors.New("read-write lock")
	}

	// write phase -- finalize changes to production zone
	err = Commit(txn, adpt)

	if err != nil {
		err = fmt.Errorf("%w : %w", ErrCommit, err)
		if wErr := Withdraw(txn.GetID(), adpt); wErr != nil {
			// encounter rollback fail, throw panic.
			// System should recovery by run tidyup
			panic(errors.Join(ErrCommit, fmt.Errorf("%w: %w", ErrRollback, wErr)))
		}
		// the dirty interval includes rollback
		txn.SetWriteTime(NextTimestamp())
		return err
	}

	txn.SetWriteTime(NextTimestamp())

	return nil
}

var timestamp atomic.Uint64

// var maxtimestamp atomic.Uint64
var muR sync.RWMutex
var muV sync.RWMutex

func NextTimestamp() uint64 {
	return timestamp.Add(1)
}

func init() {
	muR = sync.RWMutex{}
	muV = sync.RWMutex{}

}

type Set interface {
	IsDisjoint(Set) bool
	Contains(string) bool
	All() iter.Seq[string]
}

type TimeStamp comparable

type Transaction interface {
	GetID() string

	GetReadTime() uint64     // readtime will init as inf
	GetValidateTime() uint64 // validatetime will init as inf
	GetWriteTime() uint64    // writetime will init as inf

	SetReadTime(uint64)
	SetValidateTime(uint64)
	SetWriteTime(uint64)

	GetWriteSet() Set
	GetReadSet() Set
	GetTargetSet() Set

	// Read(TargetAdapter) error // aka. fetch
	// ReadTarget(string) (any, error)
	// Write(TargetAdapter, string, any) error // write one target
	Execute(TargetAdapter) error

	GetOutcomes() ([]any, error)
	GetOutcome(string) any

	// Serialize(any) ([]byte, error)
	// UnSerialize([]byte) (any, error)
}

type TransactionContainer interface {
	Add(Transaction)
	Remove(Transaction)
	GetMin() (Transaction, bool)

	IterTill(uint64, func(uint64, Transaction) bool)
}

var (
	ErrRead     = errors.New("occ: read failed")
	ErrExecute  = errors.New("occ: execution failed")
	ErrCommit   = errors.New("occ: commit failed")
	ErrConflict = errors.New("occ: validation conflict")
	ErrRollback = errors.New("occ: rollback failed")
)
