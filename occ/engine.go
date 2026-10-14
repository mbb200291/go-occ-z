// Package occ provides an optimistic concurrency control engine.
package occ

import (
	imp "github.com/mbb200291/go-occ-z/occ/internal"
	core "github.com/mbb200291/go-occ-z/occ/internal/core"
)

type Engine struct {
	prevTxn    core.TransactionContainer
	ongoingTxn core.TransactionContainer
}

func NewEngine() *Engine {
	eg := Engine{
		prevTxn:    imp.NewTransactionContainer(imp.NewSkipMap(), func(txn core.Transaction) uint64 { return txn.GetValidateTime() }),
		ongoingTxn: imp.NewTransactionContainer(imp.NewSkipMap(), func(txn core.Transaction) uint64 { return txn.GetReadTime() }),
	}
	return &eg
}

func (eg *Engine) Execute(txn *imp.Transaction) ([]any, error) {
	if err := core.Execute(txn, eg.prevTxn, eg.ongoingTxn); err != nil {
		return nil, err
	}
	return txn.GetOutcomes()
}

func (eg *Engine) TidyUp(ctx Context) error {
	return core.TidyUp(imp.GetTransactionFactory(ctx))
}

func (eg *Engine) NewTransaction(cmds []imp.Command, ctx Context) *imp.Transaction {
	return imp.NewTransaction(cmds, ctx)
}
