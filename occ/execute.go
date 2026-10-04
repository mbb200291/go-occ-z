package occ

import (
	"errors"
	"sync/atomic"
)

var timestamp atomic.Uint64

func NextTimestamp() uint64 {
	return timestamp.Add(1)
}

type Set[T comparable] interface {
	IsDisjoint(Set[T]) bool
}

type TimeStamp comparable

type Transaction interface {
	GetReadTime() uint64
	GetValidateTime() uint64
	GetWriteTime() uint64

	SetReadTime(uint64)
	SetValidateTime(uint64)
	SetWriteTime(uint64)

	GetWriteSet() Set[string]
	GetReadSet() Set[string]

	Read() error  // aka. load
	Write() error // aka. finalize
	Execute() ([]any, error)

	GetOutcomes() any
}

type TransactionContainer interface {
	Add(Transaction)
	Remove(Transaction)
	// PurgeTxnTill(uint64)
	GetMin() uint64

	IterTill(uint64, func(uint64, Transaction) bool)

	// GetTxns() []Transaction
}

func Execute(txn Transaction, prevTxns, ongoingTxns TransactionContainer) error {
	// read phase -- load read set to private zone
	txn.SetReadTime(NextTimestamp())

	// register txn to ongoing T and all T
	go ongoingTxns.Add(txn) // ordered by readtime

	txn.Read()

	// run validate
	txn.SetValidateTime(NextTimestamp())
	if outcome := Validate(txn, prevTxns, ongoingTxns.GetMin().GetReadTime()); !outcome {
		return errors.New("read-write lock")
	}

	txn.Execute()

	// write phase -- finalize changes to production zone
	txn.Write()
	txn.SetWriteTime(NextTimestamp())

	// unregister from ongoing T
	ongoingTxns.Remove(txn)

	return nil
}
