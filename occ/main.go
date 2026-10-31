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

// Context is private to one transaction attempt. Use it only during Execute.
// GetData caches successful reads; propagate read errors instead of using them
// to infer absence. SetData stages replacements for existing targets.
// Do not modify dependency sets or retain the context after Execute.
type Context = core.Context

// Command runs inside a transaction and may be executed again after a conflict.
// Keep side effects in Context; return results instead of publishing them early.
// Commands shared across Run calls must be safe for concurrent execution.
type Command = core.Command

// TargetAdapter provides concurrent storage access and backup serialization.
// Read must return independent data, including nested mutable values. Write
// must preserve unrelated targets and also support restoring a saved value.
// Copy mutable Write inputs if retaining them, rather than sharing outcome data.
// Serialization must round-trip values across process restarts.
type TargetAdapter = core.TargetAdapter

// ErrConflict marks validation conflicts. Only a bare internal conflict is retried.
var ErrConflict = core.ErrConflict

// ErrCommit reports a commit failure after a successful rollback.
var ErrCommit = core.ErrCommit

// ErrExecute wraps a command error; the transaction is not committed.
var ErrExecute = core.ErrExecute

// ErrRead wraps a storage read error returned through Context.
var ErrRead = core.ErrRead

// ErrRollback identifies rollback failure in a panic value. Stop using storage
// and recover it on restart; do not continue after recovering this panic.
var ErrRollback = core.ErrRollback

// ErrIniManager wraps configuration, lock, or startup recovery errors.
var ErrIniManager = errors.New("occ: create occ manager failed")

// ErrIniTxn reports an empty command list or a nil command.
var ErrIniTxn = errors.New("occ: init transaction failed")

// Manager coordinates transactions against one adapter and state directory.
// Obtain one with NewManager and share it across goroutines.
type Manager interface {
	// Run returns successful outcomes in command order. Failed attempts are discarded.
	// Conflicts retry, then fall back to an exclusive attempt.
	Run([]Command) ([]any, error)
	// RunExclusive waits for active Run calls and executes one attempt exclusively.
	RunExclusive([]Command) ([]any, error)
}

type occManager struct {
	runMu        sync.RWMutex
	lock         *flock.Flock
	observer     Observer
	retryOptions RetryOptions
	prevTxn      core.TransactionContainer
	ongoingTxn   core.TransactionContainer
	adpt         core.TargetAdapter
}

// NewManager acquires the working directory's .occ lock, then recovers pending
// transactions before accepting work. The lock is retained until process exit.
// Keep the working directory and its storage association unchanged.
// Pass a non-nil adapter (including no typed-nil pointer) and at most one Config.
// Omitted settings use defaults. Initialization failure releases the lock.
func NewManager(adpt TargetAdapter, configs ...Config) (*occManager, error) {
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
	if len(configs) == 1 {
		manager.observer = configs[0].Observer
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
		return mgr.RunExclusive(cmds)
	}
	return outcomes, err
}

// RunExclusive waits for active Run calls and excludes other transactions.
func (mgr *occManager) RunExclusive(cmds []Command) ([]any, error) {
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
	txn.Observer = mgr.observer
	if outcomes, err = core.Process(txn, ctx, mgr.adpt, cmds, mgr.prevTxn, mgr.ongoingTxn); err != nil {
		return nil, err
	}
	return outcomes, nil
}

func (mgr *occManager) recovery() error {
	_, err := core.Recovery(mgr.adpt)
	return err
}

// RetryOptions controls conflict retries. MaxRetries excludes the first attempt.
// BaseDelay doubles up to MaxDelay; Jitter selects 50–100% of each delay.
// With no Config: 32 retries, 1ms base, 100ms maximum, jitter disabled.
type RetryOptions = core.RetryOptions

// Config contains optional manager settings
type Config struct {
	// Retry replaces all retry defaults when non-nil. Numeric fields must be valid.
	Retry *RetryOptions
	// Observer receives events from all attempts. Nil disables notifications.
	Observer Observer
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
