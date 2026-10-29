package occ

import (
	"fmt"

	"github.com/mbb200291/go-occ-z/occ/internal/core"
)

type OccContext struct {
	outcomes []any
	dataRepo map[string]any
	readSet  Set
	writeSet Set
	adpt     core.TargetAdapter
}

func (octx *OccContext) GetOutcomes() []any {
	return octx.outcomes
}

func (octx *OccContext) GetData(target string) (any, error) {
	if _, exist := octx.dataRepo[target]; !exist {
		outcome, err := octx.adpt.Read(target)
		if err != nil {
			return nil, fmt.Errorf("%w : %w", core.ErrRead, err)
		}
		octx.dataRepo[target] = outcome
		octx.readSet.Add([]string{target})
	}
	return octx.dataRepo[target], nil
}

func (octx *OccContext) SetData(target string, data any) {
	octx.dataRepo[target] = data
	octx.writeSet.Add([]string{target})
}

func (octx *OccContext) GetReadSet() core.Set {
	return octx.readSet
}

func (octx *OccContext) GetWriteSet() core.Set {
	return octx.writeSet
}

// Append input to outcome list. Each command should only setoutcome once.
func (octx *OccContext) AppendOutcome(outcome any) {
	octx.outcomes = append(octx.outcomes, outcome)
}

func NewOccContext(adpt core.TargetAdapter) *OccContext {
	return &OccContext{
		outcomes: []any{},
		dataRepo: map[string]any{},
		adpt:     adpt,
		readSet:  NewSet(),
		writeSet: NewSet(),
	}
}
