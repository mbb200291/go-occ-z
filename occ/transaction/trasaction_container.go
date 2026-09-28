package transaction

type OrderedMap interface {
	Add(uint64, *Transaction)
	Remove(uint64)
	IterTill(uint64, func(uint64, *Transaction) bool)
	GetMin() (*Transaction, bool)
}

type TransactionContainer struct {
	om          OrderedMap
	keySelector func(*Transaction) uint64
}

func (tc *TransactionContainer) Add(txn *Transaction) {
	tc.om.Add(tc.keySelector(txn), txn)
}

func (tc *TransactionContainer) GetMin() (*Transaction, bool) {
	txn, exist := tc.om.GetMin()
	if !exist {
		return nil, false
	}
	return txn, true
}

func (tc *TransactionContainer) IterTill(
	timeStamp uint64,
	callback func(key uint64, value *Transaction) bool) {
	tc.om.IterTill(timeStamp, callback)
}

func NewTransactionContainer(om *OrderedMap, keySelector func(*Transaction) uint64) *TransactionContainer {
	return &TransactionContainer{
		om:          *om,
		keySelector: keySelector,
	}
}
