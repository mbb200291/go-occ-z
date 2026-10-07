package transaction

import (
	"math"
	"sync/atomic"

	"github.com/mbb200291/go-occ-z/occ"
)

type Context interface {
	Load(string) error
	Write(string) error
	GetOutcomes() ([]any, error)
}

type Command interface {
	IsReadOnly() bool
	IsWriteOnly() bool
	Execute(Context) error
	GetTargets() []string
}

type Transaction struct {
	cmds []Command

	readTime     atomic.Uint64
	writeTime    atomic.Uint64
	validateTime atomic.Uint64

	readSet  Set
	writeSet Set

	ctx Context
}

func NewTransaction(cmds []Command, ctx Context) *Transaction {
	txn := &Transaction{
		cmds: cmds,
		ctx:  ctx,
	}
	txn.readTime.Store(math.MaxUint64)
	txn.validateTime.Store(math.MaxUint64)
	txn.writeTime.Store(math.MaxUint64)

	for _, c := range cmds {
		if c.IsReadOnly() {
			txn.readSet.Add(c.GetTargets())
		} else if c.IsWriteOnly() {
			txn.writeSet.Add(c.GetTargets())
		} else {
			txn.readSet.Add(c.GetTargets())
			txn.writeSet.Add(c.GetTargets())
		}
	}
	return txn
}

func (txn *Transaction) GetReadTime() uint64 {
	return txn.readTime.Load()
}

func (txn *Transaction) GetWriteTime() uint64 {
	return txn.writeTime.Load()
}

func (txn *Transaction) GetValidateTime() uint64 {
	return txn.validateTime.Load()
}

func (txn *Transaction) SetReadTime(t uint64) {
	txn.readTime.Store(t)
}

func (txn *Transaction) SetWriteTime(t uint64) {
	txn.writeTime.Store(t)
}

func (txn *Transaction) SetValidateTime(t uint64) {
	txn.validateTime.Store(t)
}

func (txn *Transaction) GetWriteSet() occ.Set {
	return txn.writeSet
}

func (txn *Transaction) GetReadSet() occ.Set {
	return txn.readSet
}

var _ occ.Transaction = (*Transaction)(nil)

func (txn *Transaction) Read() error {
	for t := range txn.GetReadSet().All() {
		if err := txn.ctx.Load(t); err != nil {
			return err
		}
	}
	return nil
}

func (txn *Transaction) Write() error {
	// TODO: need be able rollback logic when fail in middle
	for t := range txn.GetWriteSet().All() {
		if err := txn.ctx.Write(t); err != nil {
			return err
		}
	}
	return nil
}

func (txn *Transaction) Execute() error { // to be overwrite
	for _, cmd := range txn.cmds {
		if err := cmd.Execute(txn.ctx); err != nil {
			return err
		}
	}
	return nil
}

func (txn *Transaction) GetOutcomes() ([]any, error) {
	return txn.ctx.GetOutcomes()
}
