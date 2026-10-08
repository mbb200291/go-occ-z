package JsonlOcc

import (
	"fmt"
	"os"

	"github.com/mbb200291/go-occ-z/occ"
	"github.com/mbb200291/go-occ-z/occ/transaction"
)

// Engine owns the transaction history for one file. Share the same Engine for
// all transactions on that file; separate engines do not share OCC history.
type Engine struct {
	path        string
	prevTxns    occ.TransactionContainer
	ongoingTxns occ.TransactionContainer
}

func NewEngine(path string) (*Engine, error) {
	normalized, err := canonicalPath(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(normalized)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("JSONL path must be a regular file: %s", normalized)
	}
	return &Engine{path: normalized, prevTxns: NewPrevTxns(), ongoingTxns: NewOngoingTxns()}, nil
}

func (engine *Engine) Execute(cmds []transaction.Command) ([]any, error) {
	if engine == nil {
		return nil, fmt.Errorf("JSONL engine is nil")
	}
	for _, cmd := range cmds {
		switch c := cmd.(type) {
		case *JsonlCommandRead:
			if c == nil {
				return nil, fmt.Errorf("nil JSONL read command")
			}
		case *JsonlCommandUpdate:
			if c == nil {
				return nil, fmt.Errorf("nil JSONL update command")
			}
		default:
			return nil, fmt.Errorf("unsupported JSONL command %T", cmd)
		}
	}
	ctx := NewJsonlContext(engine.path)
	txn := transaction.NewTransaction(cmds, ctx)
	if err := occ.Execute(txn, engine.prevTxns, engine.ongoingTxns); err != nil {
		return nil, err
	}
	return txn.GetOutcomes()
}

func newContainer(selector func(occ.Transaction) uint64) occ.TransactionContainer {
	var orderedMap transaction.OrderedMap = transaction.NewSkipMap()
	return transaction.NewTransactionContainer(&orderedMap, selector)
}

func NewOngoingTxns() occ.TransactionContainer {
	return newContainer(func(txn occ.Transaction) uint64 { return txn.GetReadTime() })
}

func NewPrevTxns() occ.TransactionContainer {
	return newContainer(func(txn occ.Transaction) uint64 { return txn.GetValidateTime() })
}
