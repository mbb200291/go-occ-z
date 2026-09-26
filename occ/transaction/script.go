package transaction

type Command interface {
	IsReadOnly() bool
	Execute() (any, error)
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
func (scpt *Script) Execute() ([]any, error) {
	outcome := []any{}
	for _, cmd := range scpt.Cmds {
		out, err := cmd.Execute()
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
		ReadSet:  set.NewSet[string](),
		WriteSet: set.NewSet[string](),
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
