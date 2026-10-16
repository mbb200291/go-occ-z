// Package occ provides an optimistic concurrency control engine.
package occ

import (
	imp "github.com/mbb200291/go-occ-z/occ/internal"
	core "github.com/mbb200291/go-occ-z/occ/internal/core"
)

type Context = imp.Context
type Command = imp.Command
type Transaction = imp.OccTransaction
type Set = core.Set

type Engine struct {
	prevTxn    core.TransactionContainer
	ongoingTxn core.TransactionContainer
	// ctxFactory func() Context
	targetAdpt core.TargetAdapter
}

func NewEngine(adpt core.TargetAdapter) *Engine {
	eg := Engine{
		prevTxn:    imp.NewTransactionContainer(imp.NewSkipMap(), func(txn core.Transaction) uint64 { return txn.GetValidateTime() }),
		ongoingTxn: imp.NewTransactionContainer(imp.NewSkipMap(), func(txn core.Transaction) uint64 { return txn.GetReadTime() }),
		targetAdpt: adpt,
	}
	return &eg
}

func (eg *Engine) Execute(cmds []Command) ([]any, error) {
	txn := imp.NewOccTransaction(cmds, imp.NewOccContext())
	if err := core.Execute(txn, eg.prevTxn, eg.ongoingTxn); err != nil {
		return nil, err
	}
	return txn.GetOutcomes()
}

func (eg *Engine) Recovery() error {
	return core.Recovery(eg.targetAdpt)
}
