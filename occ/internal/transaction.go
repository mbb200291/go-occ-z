package occ

import (
	"crypto/rand"
	"math"
	"sync/atomic"

	occ "github.com/mbb200291/go-occ-z/occ/internal/core"
)

type Context interface {
	GetOutcomes() ([]any, error)
	GetOutcome(string) any
	SetOutcome(string, any)
}

type Command interface {
	IsReadOnly() bool
	IsWriteOnly() bool
	Execute(Context) error
	GetTargets() []string
}

type OccTransaction struct {
	id   string
	cmds []Command

	readTime     atomic.Uint64
	writeTime    atomic.Uint64
	validateTime atomic.Uint64

	readSet  Set
	writeSet Set

	ctx       Context
	targetAdp occ.TargetAdapter
}

func NewOccTransaction(cmds []Command, ctx Context) *OccTransaction {
	txn := &OccTransaction{
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

func (txn *OccTransaction) GetID() string { return txn.id }

func (txn *OccTransaction) GetReadTime() uint64 {
	return txn.readTime.Load()
}

func (txn *OccTransaction) GetWriteTime() uint64 {
	return txn.writeTime.Load()
}

func (txn *OccTransaction) GetValidateTime() uint64 {
	return txn.validateTime.Load()
}

func (txn *OccTransaction) SetReadTime(t uint64) {
	txn.readTime.Store(t)
}

func (txn *OccTransaction) SetWriteTime(t uint64) {
	txn.writeTime.Store(t)
}

func (txn *OccTransaction) SetValidateTime(t uint64) {
	txn.validateTime.Store(t)
}

func (txn *OccTransaction) GetWriteSet() occ.Set {
	return txn.writeSet
}

func (txn *OccTransaction) GetReadSet() occ.Set {
	return txn.readSet
}

var _ occ.OccTransaction = (*OccTransaction)(nil)

func (txn *OccTransaction) Read() error {
	for t := range txn.GetReadSet().All() {
		data, err := txn.targetAdp.Read(t)
		if err != nil {
			return err
		}
		txn.ctx.Set(t, data)
	}
	return nil
}

func (txn *OccTransaction) ReadTarget(target string) (any, error) {
	return txn.targetAdp.Read(target)
}

func (txn *OccTransaction) Write(target string, value any) error {
	return txn.targetAdp.Write(target, value)
}

func (txn *OccTransaction) Execute() error { // to be overwrite
	for _, cmd := range txn.cmds {
		if err := cmd.Execute(txn.ctx); err != nil {
			return err
		}
	}
	return nil
}

func (txn *OccTransaction) GetOutcomes() ([]any, error) {
	return txn.ctx.GetOutcomes()
}

func (txn *OccTransaction) GetOutcome(target string) any {
	return txn.ctx.GetOutcome(target)
}

func (txn *OccTransaction) Serialize(value any) ([]byte, error) {
	return txn.targetAdp.Serialize(value)
}

func (txn *OccTransaction) UnSerialize(data []byte) (any, error) {
	return txn.targetAdp.UnSerialize(data)
}

// func GetOccTransactionFactory(ctx Context) func(string) (occ.OccTransaction, error) {
// 	return func(txnId string) (occ.OccTransaction, error) {
// 		return &OccTransaction{
// 			id:  txnId,
// 			ctx: ctx,
// 		}, nil
// 	}
// }
