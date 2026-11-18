package occ

import (
	"iter"

	occ "github.com/mbb200291/go-occ-z/occ/internal/core"
)

type Set map[string]bool

func NewSet() Set {
	return make(Set)
}

func (s Set) Add(key string, exists bool) {
	s[key] = exists
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

func (s Set) All() iter.Seq2[string, bool] {
	return func(yield func(string, bool) bool) {
		for key, exists := range s {
			if !yield(key, exists) {
				return
			}
		}
	}
}

func (s Set) Size() int {
	return len(s)
}

var _ occ.Set = Set(nil)
