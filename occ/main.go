// Package occ provides an optimistic concurrency control engine.
package occ

import (
	imp "github.com/mbb200291/go-occ-z/occ/internal"
	"github.com/mbb200291/go-occ-z/occ/internal/core"
)

type Context = core.Context
type Command = core.Command
type TargetAdapter = core.TargetAdapter

var ErrConflict = core.ErrConflict
var ErrCommit = core.ErrCommit
var ErrExecute = core.ErrExecute
var ErrRead = core.ErrRead
var ErrRollback = core.ErrRollback
var ErrIniManager = errors.New("occ: create occ manager failed")
var ErrIniTxn = errors.New("occ: init transaction failed")

type Manager struct {
	prevTxn    core.TransactionContainer
	ongoingTxn core.TransactionContainer
	ctx        Context
	adpt       core.TargetAdapter
}

func NewManager(adpt core.TargetAdapter) *Manager {
	manager := Manager{
		prevTxn:    imp.NewTransactionContainer(imp.NewSkipMap(), func(txn core.Transaction) uint64 { return txn.GetValidateTime() }),
		ongoingTxn: imp.NewTransactionContainer(imp.NewSkipMap(), func(txn core.Transaction) uint64 { return txn.GetReadTime() }),
		adpt:       adpt,
	}
	manager.recovery()
	return &manager
}

func (eg *Manager) Run(cmds []Command) ([]any, error) {
	ctx := imp.NewOccContext()
	txn := imp.NewOccTransaction(cmds, ctx)
	if err := core.Process(txn, ctx, eg.adpt, cmds, eg.prevTxn, eg.ongoingTxn); err != nil {
		return nil, err
	}
	return txn.GetOutcomes()
}

func (eg *Manager) recovery() error {
	return core.Recovery(eg.adpt)
}
