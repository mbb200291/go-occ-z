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

// data should only access / store on context object, should not directly access original source.
// should implement this.
type Context interface {
	GetOutcomes() ([]any, error)
	GetData(string) any
	SetData(string, any)
}
