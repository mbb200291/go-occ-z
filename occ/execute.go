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

type TxnContainer interface {
	AddTxn(Transaction)
	RemoveTxn(Transaction)
	PurgeTxnTill(uint64)
	GetMinTxn() uint64

	GetTxns() []Transaction
}

func Execute(txn Transaction, prevTxns, ongoingTxns TxnContainer) error {
	// register txn to ongoing T and all T
	ongoingTxns.AddTxn(txn)

	// read phase -- load read set to private zone
	txn.SetReadTime()
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
	ongoingTxns.RemoveTxn(txn)
	prevTxns.PurgeTxnTill(ongoingTxns.GetMinTxn())

	return nil
}
