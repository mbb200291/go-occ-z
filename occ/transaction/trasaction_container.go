package transaction

import "github.com/mbb200291/go-occ-z/occ"

type OrderedMap interface {
	Add(uint64, occ.Transaction)
	Remove(uint64)
	IterTill(uint64, func(key uint64, value occ.Transaction) bool)
	GetMin() (occ.Transaction, bool)
}

type TransactionContainer struct {
	om          OrderedMap
	keySelector func(occ.Transaction) uint64
}

func (tc TransactionContainer) Add(txn occ.Transaction) {
	tc.om.Add(tc.keySelector(txn), txn)
}

func (tc TransactionContainer) GetMin() (occ.Transaction, bool) {
	txn, exist := tc.om.GetMin()
	if !exist {
		return nil, false
	}
	return txn, true
}

func (tc TransactionContainer) Remove(txn occ.Transaction) {
	tc.om.Remove(tc.keySelector(txn))
}

func (tc TransactionContainer) IterTill(
	timeStamp uint64,
	callback func(key uint64, value occ.Transaction) bool) { // assumpt the ordered map can feed each item into cb. cb can early stop iteration by return false
	tc.om.IterTill(timeStamp, callback)
}

func NewTransactionContainer(om *OrderedMap, keySelector func(occ.Transaction) uint64) *TransactionContainer {
	return &TransactionContainer{
		om:          *om,
		keySelector: keySelector,
	}
}
