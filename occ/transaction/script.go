package transaction

type Command interface {
	IsReadOnly() bool
	Execute(Context) error
	GetTargets() []string
}

type Script struct {
	Cmds     []Command
	ReadSet  Set
	WriteSet Set
}

func (scpt *Script) GetReadSet() Set {
	return scpt.ReadSet
}
func (scpt *Script) GetWriteSet() Set {
	return scpt.WriteSet
}
func (scpt *Script) Execute(ctx Context) error {
	for _, cmd := range scpt.Cmds {
		err := cmd.Execute(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func NewScript(cmds []Command) *Script {
	scpt := Script{
		Cmds:     cmds,
		ReadSet:  NewSet(),
		WriteSet: NewSet(),
	}
	for _, c := range cmds {
		if c.IsReadOnly() {
			scpt.ReadSet.Add(c.GetTargets())
		} else {
			scpt.WriteSet.Add(c.GetTargets())
		}
	}
	return &scpt
}
