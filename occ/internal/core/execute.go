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
	ReadData(string) (any, bool, error) // for purpose to "get" target data
	SetData(string, any) error          // for purpose to "update" a exist target with data. Update on a not exist target is not allowed. Create target action after update also forbiden unless detele happen.
	DeleteData(string) error            // for purpose to "delete" a target with data

	GetWriteSet() Set
	GetReadSet() Set
}
