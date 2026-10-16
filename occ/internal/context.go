package occ

type BufferPoolManager interface {
	Get(string) any
	Set(string, any)
}

type OccContext struct {
	outcomes []any
	err      error
	bpm      BufferPoolManager
}

func (octx *OccContext) Write(string) error {
	return nil
}

func (octx *OccContext) Load(string) error {
	// Have to implement
	return nil
}

func (octx *OccContext) GetOutcomes() ([]any, error) {
	return octx.outcomes, octx.err
}

func (octx *OccContext) GetOutcome(target string) any {
	// TODO
	return octx.outcomes[0]
}

func (octx *OccContext) SetOutcome(target string, result any) {
	return // todo
}

func NewOccContext() *OccContext {
	return &OccContext{} // TODO
}
