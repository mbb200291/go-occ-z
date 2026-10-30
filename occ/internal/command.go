package occ

import "github.com/mbb200291/go-occ-z/occ/internal/core"

type CommandBase struct {
	readOnly  bool
	writeOnly bool
	targets   []string
}

// func (cmdb *CommandBase) IsReadOnly() bool {
// 	return cmdb.readOnly
// }

// func (cmdb *CommandBase) IsWriteOnly() bool {
// 	return cmdb.writeOnly
// }

// func (cmdb *CommandBase) GetTargets() []string {
// 	return cmdb.targets
// }

func (cmdb *CommandBase) Execute(ctx core.Context) ([]any, error) {
	// should write computation to private zone
	return nil, nil
}
