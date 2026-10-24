package occ

import (
	"crypto/rand"
	"errors"
	"math"
	"sync/atomic"

	"github.com/mbb200291/go-occ-z/occ/internal/core"
)

type OccTransaction struct {
	id   string
	cmds []core.Command

	readTime     atomic.Uint64
	writeTime    atomic.Uint64
	validateTime atomic.Uint64

	readSet  Set
	writeSet Set

	ctx       core.Context
	targetAdp core.TargetAdapter
}

func NewOccTransaction(cmds []core.Command, ctx core.Context) (*OccTransaction, error) {
	if len(cmds) == 0 {
		return nil, errors.New("zero cmmand provided")
	}
	txn := &OccTransaction{
		id:       rand.Text(), // string safe as a filename
		cmds:     cmds,
		ctx:      ctx,
		readSet:  NewSet(),
		writeSet: NewSet(),
	}
	txn.readTime.Store(math.MaxUint64)
	txn.validateTime.Store(math.MaxUint64)
	txn.writeTime.Store(math.MaxUint64)

	for _, c := range txn.cmds {
		if c == nil {
			return nil, errors.New("nil command provided")
		}
		if c.IsReadOnly() {
			txn.readSet.Add(c.GetTargets())
		} else if c.IsWriteOnly() {
			txn.writeSet.Add(c.GetTargets())
		} else {
			txn.readSet.Add(c.GetTargets())
			txn.writeSet.Add(c.GetTargets())
		}
	}
	return txn, nil
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

func (txn *OccTransaction) GetWriteSet() core.Set {
	return txn.writeSet
}

func (txn *OccTransaction) GetReadSet() core.Set {
	return txn.readSet
}
func (txn *OccTransaction) GetOutcomes() []any {
	return txn.ctx.GetOutcomes()
}

func (txn *OccTransaction) GetOutcome(target string) any {
	return txn.ctx.GetData(target)
}
