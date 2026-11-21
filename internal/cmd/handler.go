package cmd

import "io"

// Action defines the decoupled 3-step command pipeline (SRP):
// Parse validates arguments into a typed struct, Execute runs domain logic
// against Context.Store without writing to stdout, and Format renders output.
type Action[TArgs any, TResult any] interface {
	Parse(ctx *Context, args []string) (TArgs, error)
	Execute(ctx *Context, parsed TArgs) (TResult, error)
	Format(ctx *Context, w io.Writer, res TResult) error
}

// RunAction executes Parse -> Execute -> Format for an Action.
func RunAction[TArgs any, TResult any](ctx *Context, a Action[TArgs, TResult], args []string) error {
	parsed, err := a.Parse(ctx, args)
	if err != nil {
		return err
	}
	res, err := a.Execute(ctx, parsed)
	if err != nil {
		return err
	}
	return a.Format(ctx, ctx.Out, res)
}
