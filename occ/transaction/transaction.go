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

type Transaction struct {
	scpt Script

	readTime     atomic.Uint64
	writeTime    atomic.Uint64
	validateTime atomic.Uint64

	ctx Context
}

func NewTransaction(scpt *Script, ctx Context) *Transaction {
	txn := &Transaction{
		scpt: *scpt,
		ctx:  ctx,
	}
	txn.readTime.Store(math.MaxUint64)
	txn.validateTime.Store(math.MaxUint64)
	txn.writeTime.Store(math.MaxUint64)
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
	return txn.scpt.GetWriteSet()
}

func (txn *Transaction) GetReadSet() occ.Set {
	return txn.scpt.GetReadSet()
}

var _ occ.Transaction = (*Transaction)(nil)

func (txn *Transaction) Read() error {
	for t := range txn.scpt.GetReadSet().All() {
		if err := txn.ctx.Load(t); err != nil {
			return err
		}
	}
	return nil
}

func (txn *Transaction) Write() error {
	// TODO: need add rollback logic when fail in middle
	for t := range txn.scpt.GetWriteSet() {
		if err := txn.ctx.Write(t); err != nil {
			return err
		}
	}
	return nil
}

func (txn *Transaction) Execute() error { // to be overwrite
	err := txn.scpt.Execute(txn.ctx)
	return err
}

func (txn *Transaction) GetOutcomes() ([]any, error) {
	return txn.ctx.GetOutcomes()
}
