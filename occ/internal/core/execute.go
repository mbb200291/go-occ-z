package core

func Execute(ctx Context, cmds []Command) error {
	for _, cmd := range cmds {
		if err := cmd.Execute(ctx); err != nil {
			return err
		}
	}
	return nil
}

type Command interface {
	IsReadOnly() bool
	IsWriteOnly() bool
	Execute(Context) error
	GetTargets() []string
}
