package transaction

import (
	"github.com/mbb200291/go-occ-z/occ"
	"github.com/zhangyunhao116/skipmap"
)

type SkipMap struct {
	m *skipmap.Uint64Map[occ.Transaction]
}

func NewSkipMap() *SkipMap {
	return &SkipMap{
		m: skipmap.NewUint64[occ.Transaction](),
	}
}

func (s *SkipMap) Add(key uint64, value occ.Transaction) {
	s.m.Store(key, value)
}

func (s *SkipMap) Remove(key uint64) {
	s.m.Delete(key)
}

func (s *SkipMap) IterTill(
	end uint64,
	fn func(key uint64, value occ.Transaction) bool,
) {
	s.m.Range(func(key uint64, value occ.Transaction) bool {
		if key > end {
			return false
		}
		if ok := fn(key, value); !ok {
			return false
		}
		return true
	})
}

func (s *SkipMap) GetMin() (occ.Transaction, bool) {
	var min occ.Transaction
	var ok bool

	s.m.Range(func(_ uint64, value occ.Transaction) bool {
		min = value
		ok = true
		return false
	})

	return min, ok
}

var _ OrderedMap = (*SkipMap)(nil)
