package occ

import (
	"errors"
	"iter"
	"math"
	"sync/atomic"
)

var timestamp atomic.Uint64
var maxtimestamp atomic.Uint64

func NextTimestamp() uint64 {
	return timestamp.Add(1)
}

func init() {
	maxtimestamp.Store(math.MaxUint64)
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

	Read() error  // aka. load
	Write() error // aka. finalize
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
	txn.SetReadTime(NextTimestamp())
	txn.SetValidateTime(NextMaxTimestamp())
	txn.SetWriteTime(NextMaxTimestamp())

	// register txn to ongoing T and all T
	ongoingTxns.Add(txn) // ordered by readtime

	// add txn to prevTxn
	prevTxns.Add(txn)

	// unregister from ongoing T
	defer ongoingTxns.Remove(txn)

	err := txn.Read()
	if err != nil {
		return err
	}

	// set validate
	txn.SetValidateTime(NextTimestamp())

	// prepare purge write time till to
	minTxnReadTime := uint64(math.MaxInt64)
	if minTxn, exist := ongoingTxns.GetMin(); exist {
		minTxnReadTime = minTxn.GetReadTime()
	}

	// run validate
	if outcome := Validate(txn, prevTxns, minTxnReadTime); !outcome {
		return errors.New("read-write lock")
	}

	err = txn.Execute()
	if err != nil {
		return err
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
