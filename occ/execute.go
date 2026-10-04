package occ

import (
	"errors"
	"iter"
	"math"
	"sync/atomic"
)

var timestamp atomic.Uint64

func NextTimestamp() uint64 {
	return timestamp.Add(1)
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
	// PurgeTxnTill(uint64)
	GetMin() (Transaction, bool)

	IterTill(uint64, func(uint64, Transaction) bool)

	// GetTxns() []Transaction
}

func Execute(txn Transaction, prevTxns, ongoingTxns TransactionContainer) error {
	// add txn to prevTxn
	go prevTxns.Add(txn)

	// register txn to ongoing T and all T
	ongoingTxns.Add(txn) // ordered by readtime

	// unregister from ongoing T
	defer ongoingTxns.Remove(txn)

	// read phase -- load read set to private zone
	txn.SetReadTime(NextTimestamp())

	txn.Read()

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

	err := txn.Execute()
	if err != nil {
		return err
	}

	// write phase -- finalize changes to production zone
	txn.Write()
	txn.SetWriteTime(NextTimestamp())

	return nil
}
