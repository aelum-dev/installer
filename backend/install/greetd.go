package install

import "aestheticos-installer/backend/install/greetd"

// ConfigureGreetd sets up greetd as the login manager with the DMS greeter.
func ConfigureGreetd(ctx *InstallContext) error {
	exec := &greetdExecutor{ctx: ctx}
	gctx := &greetd.InstallContext{
		Username: ctx.Username,
		Logger:   ctx.Logger,
	}
	return greetd.ConfigureGreetd(exec, gctx)
}

// greetdExecutor adapts InstallContext to greetd.Executor.
type greetdExecutor struct {
	ctx *InstallContext
}

func (e *greetdExecutor) RunChroot(args ...string) ([]byte, error) {
	return runChroot(args...)
}

func (e *greetdExecutor) WriteFile(path, content string) error {
	return writeFile(path, content)
}

func (e *greetdExecutor) Logger() greetd.Logger {
	return e.ctx.Logger
}
