package transaction

type CommandBase struct {
	readOnly bool
	targets  []string
}

func (cmdb *CommandBase) IsReadOnly() bool {
	return cmdb.readOnly
}

func (cmdb *CommandBase) GetTargets() []string {
	return cmdb.targets
}

func (cmdb *CommandBase) Execute(tctx Context) (any, error) {
	return nil, nil
}
