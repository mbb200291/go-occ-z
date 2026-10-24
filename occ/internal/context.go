package occ

type OccContext struct {
	outcomes []any
	dataRepo map[string]any
}

func (octx *OccContext) GetOutcomes() []any {
	return octx.outcomes
}

func (octx *OccContext) GetData(target string) any {
	return octx.dataRepo[target]
}

func (octx *OccContext) SetData(target string, data any) {
	octx.dataRepo[target] = data
}

// func (octx *OccContext) SetErr(target string, err error) {
// 	octx.err = err
// }

// Append input to outcome list. Each command should only setoutcome once.
func (octx *OccContext) AppendOutcome(outcome any) {
	octx.outcomes = append(octx.outcomes, outcome)
}

func NewOccContext() *OccContext {
	return &OccContext{
		outcomes: []any{},
		dataRepo: map[string]any{},
	}
}
