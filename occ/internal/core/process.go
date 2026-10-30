package core

import (
	"errors"
	"fmt"
	"iter"
	"math"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"
)

// RetryProcess retries only validation conflicts. Each attempt must create a
// fresh transaction and context before calling Process.
func RetryProcess(options RetryOptions, process func() error) error {
	delay := options.BaseDelay
	for attempt := 0; ; attempt++ {
		err := process()
		// Wrapped command/storage errors must not trigger a replay.
		if err != ErrConflict || attempt >= options.MaxRetries {
			return err
		}
		time.Sleep(retryWait(delay, options.Jitter))
		delay += min(delay, options.MaxDelay-delay)
	}
}

type RetryOptions struct {
	MaxRetries int // Additional attempts after the initial attempt.
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	Jitter     bool // Randomize waits between 50% and 100% of the current delay.
}

// Keep exponential growth independent of the randomized wait.
func retryWait(delay time.Duration, jitter bool) time.Duration {
	if !jitter {
		return delay
	}
	return delay - time.Duration(rand.Int64N(int64(delay/2)+1))
}

// prevTxn should use validate time as key
// ongoingTxn should use read time as key
func Process(txn Transaction, ctx Context, adpt TargetAdapter, cmds []Command, prevTxns, ongoingTxns TransactionContainer) error {
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

	// err := Read(ctx, adpt, txn.GetReadSet())
	// if err != nil {
	// 	err = fmt.Errorf("%w : %w", ErrRead, err)
	// 	return err
	// }

	// execute transaction
	err := Execute(ctx, cmds)
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
		return ErrConflict
	}

	// write phase -- finalize changes to production zone
	if txn.GetWriteSet().Size() > 0 {
		err = Commit(txn, adpt)

		if err != nil {
			err = fmt.Errorf("%w : %w", ErrCommit, err)
			committed, wErr := Withdraw(txn.GetID(), adpt)
			if committed {
				// commit had been success
				txn.SetWriteTime(NextTimestamp())
				return nil
			}
			if wErr != nil {
				// encounter rollback fail, throw panic.
				// System should recovery by run tidyup
				panic(errors.Join(ErrCommit, fmt.Errorf("%w: %w", ErrRollback, wErr), err))
			}
			// the dirty interval includes rollback
			txn.SetWriteTime(NextTimestamp())
			return err
		}
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
	Size() int
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

	GetOutcomes() []any
	GetWriteData(string) any
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
