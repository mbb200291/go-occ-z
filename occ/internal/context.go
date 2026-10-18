package occ

// type BufferPoolManager interface {
// 	Get(string) any
// 	Set(string, any)
// }

type OccContext struct {
	outcomes []any
	err      error
	dataRepo map[string]any
}

func (octx *OccContext) GetOutcomes() ([]any, error) {
	return octx.outcomes, octx.err
}

func (octx *OccContext) GetOutcome(target string) any {
	return octx.dataRepo[target]
}

// Append input to outcome list. Each command should only setoutcome once.
func (octx *OccContext) SetOutcome(target string, outcome any) {
	octx.dataRepo[target] = outcome
	octx.outcomes = append(octx.outcomes, outcome)
}

func (octx *OccContext) SetErr(target string, err error) {
	octx.err = err
}

func NewOccContext() *OccContext {
	return &OccContext{
		outcomes: []any{},
		dataRepo: map[string]any{},
	}
}
