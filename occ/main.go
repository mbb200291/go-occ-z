// Package occ provides an optimistic concurrency control engine.
package occ

import (
	"errors"
	"fmt"
	"time"

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
	retryOptions RetryOptions
	prevTxn      core.TransactionContainer
	ongoingTxn   core.TransactionContainer
	adpt         core.TargetAdapter
}

// NewManager requires a non-nil adapter
// Must not pass typed-nil adapter
// Omitted settings use defaults
func NewManager(adpt core.TargetAdapter, configs ...Config) (*occManager, error) {
	if adpt == nil {
		err := fmt.Errorf("%w : nil adaptor", ErrIniManager)
		return nil, err

	}
	options, err := resolveConfig(configs)
	if err != nil {
		return nil, err
	}
	manager := occManager{
		retryOptions: options,
		prevTxn:      imp.NewTransactionContainer(imp.NewSkipMap(), func(txn core.Transaction) uint64 { return txn.GetValidateTime() }),
		ongoingTxn:   imp.NewTransactionContainer(imp.NewSkipMap(), func(txn core.Transaction) uint64 { return txn.GetReadTime() }),
		adpt:         adpt,
	}
	if err := manager.recovery(); err != nil {
		return nil, fmt.Errorf("%w: recovery: %w", ErrIniManager, err)
	}
	return &manager, nil
}

func (mgr *occManager) Run(cmds []Command) ([]any, error) {
	var outcomes []any
	err := core.RetryProcess(mgr.retryOptions, func() error {
		ctx := imp.NewOccContext()
		txn, err := imp.NewOccTransaction(cmds, ctx)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrIniTxn, err)
		}
		if err := core.Process(txn, ctx, mgr.adpt, cmds, mgr.prevTxn, mgr.ongoingTxn); err != nil {
			return err
		}
		outcomes = txn.GetOutcomes()
		return nil
	})
	return outcomes, err
}

func (mgr *occManager) recovery() error {
	_, err := core.Recovery(mgr.adpt)
	return err
}

type RetryOptions = core.RetryOptions

// Config contains optional manager settings
type Config struct {
	Retry *RetryOptions
}

func resolveConfig(configs []Config) (RetryOptions, error) {
	options := RetryOptions{
		MaxRetries: 32, BaseDelay: time.Millisecond, MaxDelay: 100 * time.Millisecond}
	if len(configs) > 1 {
		return options, fmt.Errorf("%w: expected at most one Config", ErrIniManager)
	}
	if len(configs) == 1 && configs[0].Retry != nil {
		options = *configs[0].Retry
	}
	if options.MaxRetries < 0 || options.BaseDelay <= 0 || options.MaxDelay < options.BaseDelay {
		return options, fmt.Errorf("%w: retry requires MaxRetries >= 0 and 0 < BaseDelay <= MaxDelay", ErrIniManager)
	}
	return options, nil
}
