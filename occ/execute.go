package occ

import "errors"

type Set[T comparable] interface {
	IsDisjoint(Set[T]) bool
}

type TimeStamp comparable

type Transaction interface {
	GetReadTime() uint64
	GetValidateTime() uint64
	GetWriteTime() uint64

	SetReadTime()
	SetValidateTime()
	SetWriteTime()

	GetWriteSet() Set[string]
	GetReadSet() Set[string]

	Read() error  // aka. load
	Write() error // aka. finalize
	Execute() ([]any, error)

	GetOutcome() any
}

type TransactionContainer interface {
	Add(Transaction)
	Remove(Transaction)
	// PurgeTxnTill(uint64)
	GetMin() uint64

	IterTill(uint64, func(*Transaction))

	// GetTxns() []Transaction
}

func Execute(txn Transaction, prevTxns, ongoingTxns TransactionContainer) error {
	// read phase -- load read set to private zone
	txn.SetReadTime()

	// register txn to ongoing T and all T
	go ongoingTxns.Add(txn) // ordered by readtime

	txn.Read()

	// run validate
	txn.SetValidateTime()
	if outcome := Validate(txn, prevTxns); !outcome {
		return errors.New("read-write lock")
	}

	txn.Execute()

	// write phase -- finalize changes to production zone
	txn.Write()
	txn.SetWriteTime()

	// unregister from ongoing T
	ongoingTxns.Remove(txn)
	// prevTxns.PurgeTxnTill(ongoingTxns.GetMin())

	return nil
}
