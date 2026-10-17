package core

func read(ctx Context, adpt TargetAdapter, target string) error {
	data, err := adpt.Read(target)
	if err != nil {
		return err
	}
	ctx.SetOutcome(target, data)
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

type Context interface {
	GetOutcomes() ([]any, error)
	GetOutcome(string) any
	SetOutcome(string, any)
}
