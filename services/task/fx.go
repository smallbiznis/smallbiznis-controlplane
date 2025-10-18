package task

import (
	"go.uber.org/fx"
)

var Module = fx.Module("task.service",
	fx.Provide(
		NewService,
	),
	fx.Invoke(NewScheduler),
)
