package transaction

import "math"

type Context interface {
	Load(any) error
	Write(any) error
	GetOutcomes() ([]any, error)
}

type Transaction struct {
	scpt Script

	ReadTime     uint64
	WriteTime    uint64
	ValidateTime uint64

	ctx Context
}

func NewTransaction(scpt *Script) *Transaction {
	txn := Transaction{
		scpt:         *scpt,
		ReadTime:     uint64(math.MaxUint64),
		ValidateTime: uint64(math.MaxUint64),
		WriteTime:    uint64(math.MaxUint64),
	}
	return &txn
}

func (txn *Transaction) GetReadTime() uint64 {
	return txn.ReadTime
}

func (txn *Transaction) GetWriteTime() uint64 {
	return txn.WriteTime
}

func (txn *Transaction) GetValidateTime() uint64 {
	return txn.ValidateTime
}

func (txn *Transaction) SetReadTime(t uint64) {
	txn.ReadTime = t
}

func (txn *Transaction) SetWriteTime(t uint64) {
	txn.WriteTime = t
}

func (txn *Transaction) SetValidateTime(t uint64) {
	txn.ValidateTime = t
}

func (txn *Transaction) GetWriteSet() Set[string] {
	return txn.scpt.GetWriteSet()
}

func (txn *Transaction) GetReadSet() Set[string] {
	return txn.scpt.GetReadSet()
}

func (txn *Transaction) Read() error {
	for t := range txn.scpt.GetReadSet() {
		if err := txn.ctx.Load(t); err != nil {
			return err
		}
	}
	return nil
}

func (txn *Transaction) Write() error {
	for t := range txn.scpt.GetWriteSet() {
		if err := txn.ctx.Write(t); err != nil {
			return err
		}
	}
	return nil
}

func (txn *Transaction) Execute() ([]any, error) { // to be overwrite
	return txn.scpt.Execute(txn.ctx)
}

func (txn *Transaction) GetOutcomes() ([]any, error) {
	return txn.ctx.GetOutcomes()
}
