package transaction

import (
	"github.com/zhangyunhao116/skipmap"
)

type SkipMap struct {
	m *skipmap.Uint64Map[*Transaction]
}

func NewSkipMap() *SkipMap {
	return &SkipMap{
		m: skipmap.NewUint64[*Transaction](),
	}
}

func (s *SkipMap) Add(key uint64, value *Transaction) {
	s.m.Store(key, value)
}

func (s *SkipMap) Delete(key uint64) {
	s.m.Delete(key)
}

func (s *SkipMap) IterTill(
	end uint64,
	fn func(key uint64, value *Transaction) bool,
) {
	s.m.Range(func(key uint64, value *Transaction) bool {
		if key > end {
			return false
		}
		if ok := fn(key, value); !ok {
			return false
		}
		return true
	})
}

func (s *SkipMap) GetMin() (*Transaction, bool) {
	var min *Transaction
	var ok bool

	s.m.Range(func(_ uint64, value *Transaction) bool {
		min = value
		ok = true
		return false
	})

	return min, ok
}

var _ OrderedMap = (*SkipMap)(nil)
