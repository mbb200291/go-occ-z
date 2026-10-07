package transaction

type ContextBase struct {
	outcomes []any
	err      error
}

func (ctxb *ContextBase) Write(string) error {
	return nil
}

func (ctxb *ContextBase) Load(string) error {
	// Have to implement
	return nil
}

func (ctxb *ContextBase) Backup(string) error {
	// Have to implement
	return nil
}

func (ctxb *ContextBase) Revert(string) error {
	// Have to implement
	return nil
}

func (ctxb *ContextBase) DiscardBackup(string) error {
	// Have to implement
	return nil
}

func (ctxb *ContextBase) GetOutcomes() ([]any, error) {
	return ctxb.outcomes, ctxb.err
}
