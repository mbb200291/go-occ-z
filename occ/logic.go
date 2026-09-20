package occ

import "errors"

type Transaction interface {
	GetReadTime() float64
	GetValidateTime() float64
	GetWriteTime() float64

	SetReadTime()
	SetValidateTime()
	SetWriteTime()

	GetWriteSet() Set[string]
	GetReadSet() Set[string]

	Read() error
	Write() error
}

type Operation interface {
	AsTxn() Transaction
}

type TxnContainer interface {
	AddTxn(Transaction)
	RemoveTxn(Transaction)
	PurgeTxnTill(float64)
	GetMinReadTimeTxn() Transaction

	GetTxns() []Transaction
}

func Execute(op Operation, prevTxn, ongoingTxn TxnContainer) error {
	// init txn from op
	txn := op.AsTxn()

	// register txn to ongoing T and all T
	txn.SetReadTime()
	ongoingTxn.AddTxn(txn)

	// read phase
	txn.Read()

	// run validate
	txn.SetValidateTime()
	outcome := Validate(txn, prevTxn)
	if !outcome {
		return errors.New("read-write lock")
	}

	// write phase
	txn.Write()
	txn.SetWriteTime()

	// unregister from ongoing T
	ongoingTxn.RemoveTxn(txn)
	prevTxn.PurgeTxnTill(txn.GetReadTime())

	return nil
}
