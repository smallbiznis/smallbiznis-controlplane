package bootstrap

import (
	"context"

	"go.uber.org/fx"
	"gorm.io/gorm"
)

var Module = fx.Module("bootstrap",
	fx.Provide(
		NewService,
	),
	fx.Invoke(runBootstrap),
)

// Run after DB initialized
func runBootstrap(lc fx.Lifecycle, b *Service, db *gorm.DB) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			// Execute bootstrap logic
			b.Migrate()
			return nil
		},
	})
}
