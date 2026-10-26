package core

func read(ctx Context, adpt TargetAdapter, target string) error {
	data, err := adpt.Read(target)
	if err != nil {
		return err
	}
	ctx.SetData(target, data)
	return nil
}

func Read(ctx Context, adpt TargetAdapter, targets Set) error {
	for t := range targets.All() {
		if err := read(ctx, adpt, t); err != nil {
			return err
		}
	}
	return nil
}

// Data should only access and store on context object, should not directly access original source.
// Should be implemented by client.
type Context interface {
	GetOutcomes() []any
	GetData(string) any
	SetData(string, any)
	AppendOutcome(any)
}
