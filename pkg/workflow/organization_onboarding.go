package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func OrganizationOnboarding(ctx workflow.Context) error {
	// fields := []zap.Field{
	// 	zap.String("organization_id", req.OrganizationId),
	// }

	// zap.L().With(fields...).Info("Incoming Request")

	// info := workflow.GetInfo(ctx)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    3,
		},
	}

	ctx = workflow.WithActivityOptions(ctx, ao)

	// if err := workflow.ExecuteActivity(ctx, activities.OnboardOrganization, req).Get(ctx, nil); err != nil {
	// 	zap.L().With(fields...).Error("failed to ExecuteActivity", zap.Error(err), zap.String("activities", "OnboardOrganization"))
	// 	return err
	// }

	return nil
}
