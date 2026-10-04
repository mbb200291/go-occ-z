package transaction

type ContextBase struct {
	outcomes []any
	err      error
}

func (ctxb *ContextBase) Write(any) error {
	return nil
}

func (ctxb *ContextBase) Load(any) error {
	return nil
}

func (ctxb *ContextBase) GetOutcomes() ([]any, error) {
	return ctxb.outcomes, ctxb.err
}
