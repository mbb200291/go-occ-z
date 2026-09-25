package transaction

import (
	set "github.com/mbb200291/go-occ-z/occ/set"
)

type Context interface {
	Load(any) error
	Write(any) error
}

type Transaction struct {
	scpt Script

	ReadTime     float64
	WriteTime    float64
	ValidateTime float64

	Outcome []any

	Ctx Context
}

func NewTransaction(scpt *Script) *Transaction {
	txn := Transaction{
		scpt: *scpt,
	}
	return &txn
}

func (txn *Transaction) GetReadTime() float64 {
	return txn.ReadTime
}

func (txn *Transaction) GetWriteTime() float64 {
	return txn.WriteTime
}

func (txn *Transaction) GetValidateTime() float64 {
	return txn.ValidateTime
}

func (txn *Transaction) SetReadTime(t float64) {
	txn.ReadTime = t
}

func (txn *Transaction) SetWriteTime(t float64) {
	txn.WriteTime = t
}

func (txn *Transaction) SetValidateTime(t float64) {
	txn.ValidateTime = t
}

func (txn *Transaction) GetWriteSet() set.Set[string] {
	return txn.scpt.GetWriteSet()
}

func (txn *Transaction) GetReadSet() set.Set[string] {
	txn.scpt.GetReadSet()
}

func (txn *Transaction) Read() error {
	for _, t := range txn.scpt.GetReadSet() {
		if err := txn.Ctx.Load(t); err != nil {
			return err
		}
	}
	return nil
}

func (txn *Transaction) Write() error {
	for _, t := range txn.scpt.GetWriteSet() {
		if err := txn.Ctx.Write(t); err != nil {
			return err
		}
	}
	return nil
}

func (txn *Transaction) Execute() ([]any, error) { // to be overwrite
	return txn.scpt.Execute()
}

func (txn *Transaction) GetOutcome() any {
	return txn.Outcome
}
