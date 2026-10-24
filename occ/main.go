// Package occ provides an optimistic concurrency control engine.
package occ

import (
	"errors"
	"fmt"

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

type Manager interface {
	Run([]Command) ([]any, error)
}

type occManager struct {
	prevTxn    core.TransactionContainer
	ongoingTxn core.TransactionContainer
	adpt       core.TargetAdapter
}

// NewManager requires a non-nil adapter. Callers must not pass typed-nil
// adapters or commands
func NewManager(adpt core.TargetAdapter) (*occManager, error) {
	if adpt == nil {
		err := fmt.Errorf("%w : nil adaptor", ErrIniManager)
		return nil, err

	}
	manager := occManager{
		prevTxn:    imp.NewTransactionContainer(imp.NewSkipMap(), func(txn core.Transaction) uint64 { return txn.GetValidateTime() }),
		ongoingTxn: imp.NewTransactionContainer(imp.NewSkipMap(), func(txn core.Transaction) uint64 { return txn.GetReadTime() }),
		adpt:       adpt,
	}
	if err := manager.recovery(); err != nil {
		return nil, fmt.Errorf("%w: recovery: %w", ErrIniManager, err)
	}
	return &manager, nil
}

func (mgr *occManager) Run(cmds []Command) ([]any, error) {
	ctx := imp.NewOccContext()
	txn, err := imp.NewOccTransaction(cmds, ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrIniTxn, err)
	}
	if err := core.Process(txn, ctx, mgr.adpt, cmds, mgr.prevTxn, mgr.ongoingTxn); err != nil {
		return nil, err
	}
	return txn.GetOutcomes(), nil
}

func (mgr *occManager) recovery() error {
	_, err := core.Recovery(mgr.adpt)
	return err
}
