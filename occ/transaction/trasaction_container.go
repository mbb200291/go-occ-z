package transaction

type OrderedMap interface {
	Add(uint64, *Transaction)
	Delete(uint64)
	MapTill(uint64, func(key uint64, value *Transaction) bool)
	GetMin() (*Transaction, bool)
}

type TransactionContainer struct {
	om          OrderedMap
	keySelector func(*Transaction) uint64
}

func (tc *TransactionContainer) Add(txn *Transaction) {
	tc.om.Add(tc.keySelector(txn), txn)
}

func (tc *TransactionContainer) GetMinTxn() *Transaction {
	txn, exist := tc.om.GetMin()
	if !exist {
		return nil
	}
	return txn
}

func NewTransactionContainer() *TransactionContainer {

}
