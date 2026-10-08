package JsonlOcc

import (
	"fmt"
	"strconv"

	"github.com/mbb200291/go-occ-z/occ/transaction"
)

type JsonlCommand struct {
	targets  []string
	readOnly bool
}

func (cmd *JsonlCommand) IsReadOnly() bool { return cmd.readOnly }

// Updates must read the old record to preserve fields outside the JSON path.
func (cmd *JsonlCommand) IsWriteOnly() bool    { return false }
func (cmd *JsonlCommand) GetTargets() []string { return append([]string(nil), cmd.targets...) }

func (cmd *JsonlCommand) context(base transaction.Context) (*JsonlContext, uint, error) {
	ctx, ok := base.(*JsonlContext)
	if !ok || ctx == nil {
		return nil, 0, fmt.Errorf("JSONL command requires *JsonlContext")
	}
	if ctx.err != nil {
		return ctx, 0, ctx.err
	}
	if len(cmd.targets) != 1 {
		return ctx, 0, ctx.fail(fmt.Errorf("JSONL command requires exactly one target line"))
	}
	line, err := parseLine(cmd.targets[0])
	if err != nil {
		return ctx, 0, ctx.fail(err)
	}
	return ctx, line, nil
}

type JsonlCommandRead struct {
	JsonlCommand
	JsonPath string
}

func NewRead(line uint, path string) *JsonlCommandRead {
	return &JsonlCommandRead{JsonlCommand: JsonlCommand{targets: []string{strconv.FormatUint(uint64(line), 10)}, readOnly: true}, JsonPath: path}
}

func (cmd *JsonlCommandRead) Execute(base transaction.Context) error {
	ctx, line, err := cmd.context(base)
	if err != nil {
		return err
	}
	record, err := ctx.record(line)
	if err != nil {
		return err
	}
	tokens, err := parsePath(cmd.JsonPath)
	if err != nil {
		return ctx.fail(err)
	}
	value, err := lookupPath(record, tokens)
	if err != nil {
		return ctx.fail(err)
	}
	return ctx.appendOutcome(value)
}

type JsonlCommandUpdate struct {
	JsonlCommand
	JsonPath string
	Value    any
}

func NewUpdate(line uint, path string, value any) *JsonlCommandUpdate {
	return &JsonlCommandUpdate{JsonlCommand: JsonlCommand{targets: []string{strconv.FormatUint(uint64(line), 10)}}, JsonPath: path, Value: value}
}

func (cmd *JsonlCommandUpdate) Execute(base transaction.Context) error {
	ctx, line, err := cmd.context(base)
	if err != nil {
		return err
	}
	record, err := ctx.record(line)
	if err != nil {
		return err
	}
	tokens, err := parsePath(cmd.JsonPath)
	if err != nil {
		return ctx.fail(err)
	}
	value, err := cloneJSON(cmd.Value)
	if err != nil {
		return ctx.fail(fmt.Errorf("invalid update value: %w", err))
	}
	if len(tokens) == 0 {
		record = value
	} else {
		parent, err := lookupPath(record, tokens[:len(tokens)-1])
		if err != nil {
			return ctx.fail(err)
		}
		last := tokens[len(tokens)-1]
		// Existing paths only; no implicit object or array creation.
		if _, err := lookupPath(parent, []pathToken{last}); err != nil {
			return ctx.fail(err)
		}
		if last.array {
			parent.([]any)[last.index] = value
		} else {
			parent.(map[string]any)[last.key] = value
		}
	}
	ctx.records[line] = record
	return ctx.appendOutcome(value)
}

var _ transaction.Command = (*JsonlCommandRead)(nil)
var _ transaction.Command = (*JsonlCommandUpdate)(nil)
