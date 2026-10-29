package core

func Execute(ctx Context, cmds []Command) error {
	for _, cmd := range cmds {
		if err := cmd.Execute(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Should implemented by client.
type Command interface {
	// IsReadOnly() bool
	// IsWriteOnly() bool
	Execute(Context) error
	// GetTargets() []string
}

// Data should only access and store on context object, should not directly access original source.
type Context interface {
	GetOutcomes() []any
	GetData(string) (any, error)
	SetData(string, any)
	AppendOutcome(any)

	GetWriteSet() Set
	GetReadSet() Set
}
