package core

func Execute(ctx Context, cmds []Command) ([]any, error) {
	var outcomes []any
	var outcome any
	var err error
	for _, cmd := range cmds {
		if outcome, err = cmd.Execute(ctx); err != nil {
			return nil, err
		}
		outcomes = append(outcomes, outcome)
	}
	return outcomes, nil
}

// Should implemented by client.
type Command interface {
	Execute(Context) (any, error)
}

// Data should only access and store on context object, should not directly access original source.
type Context interface {
	GetData(string) (any, error)
	SetData(string, any)

	GetWriteSet() Set
	GetReadSet() Set
}
