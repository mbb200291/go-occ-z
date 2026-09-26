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

func (s *SkipMap) MapTill(
	end uint64,
	fn func(key uint64, value *Transaction) bool,
) {
	s.m.Range(func(key uint64, value *Transaction) bool {
		if key > end {
			return false
		}

		return fn(key, value)
	})
}

func (s *SkipMap) GetMin() (*Transaction, bool) {
	var min *Transaction

	s.m.Range(func(_ uint64, value *Transaction) bool {
		min = value
		return false
	})

	return min, true
}

var _ OrderedMap = (*SkipMap)(nil)
