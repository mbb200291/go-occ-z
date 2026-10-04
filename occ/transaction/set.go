package transaction

import (
	"iter"

	"github.com/mbb200291/go-occ-z/occ"
)

type Set map[string]struct{}

func NewSet() Set {
	return make(Set)
}

func (s Set) Add(keys []string) {
	for _, key := range keys {
		s[key] = struct{}{}
	}
}

func (s Set) Remove(key string) {
	delete(s, key)
}

func (s Set) Contains(key string) bool {
	_, exists := s[key]
	return exists
}

func (s Set) IsDisjoint(other occ.Set) bool {
	for key := range s {
		if other.Contains(key) {
			return false
		}
	}
	return true
}

func (s Set) All() iter.Seq[string] {
	return func(yield func(string) bool) {
		for key := range s {
			if !yield(key) {
				return
			}
		}
	}
}

var _ occ.Set = Set(nil)
