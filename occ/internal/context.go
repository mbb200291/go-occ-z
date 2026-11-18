package occ

import (
	"fmt"

	"github.com/mbb200291/go-occ-z/occ/internal/core"
)

type cachedTarget struct {
	value  any
	exists bool
}

type OccContext struct {
	dataRepo map[string]cachedTarget
	readSet  Set
	writeSet Set
	adpt     core.TargetAdapter
}

// if data not in cache, read from source
// if data in cache, read from cache
func (octx *OccContext) ReadData(target string) (any, bool, error) {
	if _, exist := octx.dataRepo[target]; !exist {
		data, exist, err := octx.adpt.Read(target)
		if err != nil {
			return nil, false, fmt.Errorf("%w : %w", core.ErrRead, err)
		}
		octx.dataRepo[target] = cachedTarget{value: data, exists: exist} // exist will be updated
		octx.readSet.Add(target, true)                                   // second value is not used
	}
	cached := octx.dataRepo[target]
	return cached.value, cached.exists, nil
}

// 1. cache data will replace by provided data
// 2. data will either create or replace target data
func (octx *OccContext) SetData(target string, data any) error { // will not read target first to determine existenc
	octx.dataRepo[target] = cachedTarget{value: data, exists: true}
	octx.writeSet.Add(target, true)
	return nil
}

// DeleteData stages absence and remembers it for subsequent reads.
func (octx *OccContext) DeleteData(target string) error {
	octx.dataRepo[target] = cachedTarget{}
	octx.writeSet.Add(target, false)
	return nil
}

func (octx *OccContext) GetReadSet() core.Set {
	return octx.readSet
}

func (octx *OccContext) GetWriteSet() core.Set {
	return octx.writeSet
}

func NewOccContext(adpt core.TargetAdapter) *OccContext {
	return &OccContext{
		dataRepo: map[string]cachedTarget{},
		adpt:     adpt,
		readSet:  NewSet(),
		writeSet: NewSet(),
	}
}
