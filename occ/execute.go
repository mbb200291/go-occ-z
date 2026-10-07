package occ

import (
	"errors"
	"iter"
	"math"
	"sync"
	"sync/atomic"
)

var timestamp atomic.Uint64
var maxtimestamp atomic.Uint64
var muR sync.RWMutex
var muV sync.RWMutex

func NextTimestamp() uint64 {
	return timestamp.Add(1)
}

func init() {
	maxtimestamp.Store(math.MaxUint64)
	muR = sync.RWMutex{}
	muV = sync.RWMutex{}

}

func NextMaxTimestamp() uint64 {
	return maxtimestamp.Add(^uint64(0))
}

type Set interface {
	IsDisjoint(Set) bool
	Contains(string) bool
	All() iter.Seq[string]
}

type TimeStamp comparable

type Transaction interface {
	GetReadTime() uint64     // readtime will init as inf
	GetValidateTime() uint64 // validatetime will init as inf
	GetWriteTime() uint64    // writetime will init as inf

	SetReadTime(uint64)
	SetValidateTime(uint64)
	SetWriteTime(uint64)

	GetWriteSet() Set
	GetReadSet() Set

	Read() error  // aka. fetch
	Write() error // aka. commit
	Execute() error

	GetOutcomes() ([]any, error)
}

type TransactionContainer interface {
	Add(Transaction)
	Remove(Transaction)
	GetMin() (Transaction, bool)

	IterTill(uint64, func(uint64, Transaction) bool)
}

// prevTxn should use validate time as key
// ongoingTxn should use read time as key
func Execute(txn Transaction, prevTxns, ongoingTxns TransactionContainer) error {
	// read phase -- load read set to private zone
	muR.Lock() // [+] lock
	txn.SetReadTime(NextTimestamp())

	// register txn to ongoing T and all T
	ongoingTxns.Add(txn) // ordered by readtime
	muR.Unlock()         // [-] unlock

	// unregister txn from ongoing T after txn complete
	defer ongoingTxns.Remove(txn)

	err := txn.Read()
	if err != nil {
		return err
	}

	// execute transaction optimicaly
	err = txn.Execute()
	if err != nil {
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
		return errors.New("read-write lock")
	}

	// write phase -- finalize changes to production zone
	err = txn.Write()
	if err != nil {
		prevTxns.Remove(txn)
		return err
	}

	txn.SetWriteTime(NextTimestamp())

	return nil
}
