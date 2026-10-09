package transaction

import (
	"crypto/rand"
	"math"
	"sync/atomic"

	"github.com/mbb200291/go-occ-z/occ"
)

type Context interface {
	Load(string) error
	ReadTarget(string) (any, error) // reads shared data for backup without changing the private workspace.
	Write(string, any) error
	GetOutcomes() ([]any, error)
	GetOutcome(string) any
	Serialize(any) ([]byte, error)
	UnSerialize([]byte) (any, error)
	Revert(string) error
}

type Command interface {
	IsReadOnly() bool
	IsWriteOnly() bool
	Execute(Context) error
	GetTargets() []string
}

type Transaction struct {
	id   string
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
		id:       rand.Text(),
		cmds:     append([]Command(nil), cmds...),
		ctx:      ctx,
		readSet:  NewSet(),
		writeSet: NewSet(),
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

func (txn *Transaction) GetID() string { return txn.id }

func (txn *Transaction) Revert(target string) error {
	return txn.ctx.Revert(target)
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

func (txn *Transaction) ReadTarget(target string) (any, error) {
	return txn.ctx.ReadTarget(target)
}

func (txn *Transaction) Write(target string, value any) error {
	return txn.ctx.Write(target, value)
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

func (txn *Transaction) GetOutcome(target string) any {
	return txn.ctx.GetOutcome(target)
}

func (txn *Transaction) Serialize(value any) ([]byte, error) {
	return txn.ctx.Serialize(value)
}

func (txn *Transaction) UnSerialize(data []byte) (any, error) {
	return txn.ctx.UnSerialize(data)
}
