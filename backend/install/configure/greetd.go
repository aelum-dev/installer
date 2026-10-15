package configure

import (
	"aelum-installer/backend/install/greetd"
	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// greetd configuration
// ============================================================================

// ConfigureGreetd sets up greetd as the login manager with the DMS greeter.
func ConfigureGreetd(ctx *shared.InstallContext) error {
	exec := &greetdExecutor{ctx: ctx}
	gctx := &greetd.InstallContext{
		Username: ctx.Username,
		Logger:   ctx.Logger,
	}
	return greetd.ConfigureGreetd(exec, gctx)
}

type greetdExecutor struct {
	ctx *shared.InstallContext
}

func (e *greetdExecutor) RunChroot(args ...string) ([]byte, error) {
	return primitives.RunChroot(args...)
}

func (e *greetdExecutor) WriteFile(path, content string) error {
	return primitives.WriteFile(path, content)
}

func (e *greetdExecutor) Logger() shared.Logger {
	return e.ctx.Logger
}
