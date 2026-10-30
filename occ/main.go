// Package occ provides an optimistic concurrency control engine.
package occ

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gofrs/flock"

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
	ExcRun([]Command) ([]any, error)
}

type occManager struct {
	runMu        sync.RWMutex
	lock         *flock.Flock
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
	lock, err := acquireLock()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrIniManager, err)
	}
	manager := occManager{
		lock:         lock,
		retryOptions: options,
		prevTxn:      imp.NewTransactionContainer(imp.NewSkipMap(), func(txn core.Transaction) uint64 { return txn.GetValidateTime() }),
		ongoingTxn:   imp.NewTransactionContainer(imp.NewSkipMap(), func(txn core.Transaction) uint64 { return txn.GetReadTime() }),
		adpt:         adpt,
	}
	if err := manager.recovery(); err != nil {
		return nil, errors.Join(fmt.Errorf("%w: recovery: %w", ErrIniManager, err), lock.Close())
	}
	retainLock(lock)
	return &manager, nil
}

func (mgr *occManager) Run(cmds []Command) ([]any, error) {
	outcomes, err := func() ([]any, error) {
		mgr.runMu.RLock()
		defer mgr.runMu.RUnlock()
		return mgr.run(cmds)
	}()
	// retry execution under exclusive lock to rescue starving transaction
	if err == ErrConflict {
		return mgr.ExcRun(cmds)
	}
	return outcomes, err
}

// ExcRun waits for active Run calls and excludes other transactions.
func (mgr *occManager) ExcRun(cmds []Command) ([]any, error) {
	mgr.runMu.Lock()
	defer mgr.runMu.Unlock()
	return mgr.runOnce(cmds)
}

func (mgr *occManager) run(cmds []Command) ([]any, error) {
	var outcomes []any
	err := core.RetryProcess(mgr.retryOptions, func() error {
		var err error
		outcomes, err = mgr.runOnce(cmds)
		return err
	})
	return outcomes, err
}

func (mgr *occManager) runOnce(cmds []Command) ([]any, error) {
	ctx := imp.NewOccContext(mgr.adpt)
	txn, err := imp.NewOccTransaction(cmds, ctx)
	var outcomes []any
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrIniTxn, err)
	}
	if outcomes, err = core.Process(txn, ctx, mgr.adpt, cmds, mgr.prevTxn, mgr.ongoingTxn); err != nil {
		return nil, err
	}
	return outcomes, nil
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
