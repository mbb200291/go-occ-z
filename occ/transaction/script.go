package transaction

type Command interface {
	IsReadOnly() bool
	Execute(Context) (any, error)
	GetTarget() string
}

type Script struct {
	Cmds     []Command
	ReadSet  Set[string]
	WriteSet Set[string]
}

func (scpt *Script) GetReadSet() Set[string] {
	return scpt.ReadSet
}
func (scpt *Script) GetWriteSet() Set[string] {
	return scpt.WriteSet
}
func (scpt *Script) Execute(ctx Context) ([]any, error) {
	outcome := []any{}
	for _, cmd := range scpt.Cmds {
		out, err := cmd.Execute(ctx)
		if err != nil {
			return nil, err
		}
		outcome = append(outcome, out)
	}
	return outcome, nil
}

func NewScript(cmds []Command) *Script {
	scpt := Script{
		Cmds:     cmds,
		ReadSet:  NewSet[string](),
		WriteSet: NewSet[string](),
	}
	for _, c := range cmds {
		if c.IsReadOnly() {
			scpt.ReadSet.Add(c.GetTarget())
		} else {
			scpt.WriteSet.Add(c.GetTarget())
		}
	}
	return &scpt
}
