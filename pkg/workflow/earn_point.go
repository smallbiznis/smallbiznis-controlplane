package workflow

// pointv1 "github.com/smallbiznis/go-genproto/smallbiznis/loyalty/point/v1"

// func EarnPoint(ctx workflow.Context, req *pointv1.EarningRequest) error {
// 	fields := []zap.Field{
// 		zap.String("organization_id", req.OrganizationId),
// 		zap.String("user_id", req.UserId),
// 		zap.String("reference_id", req.ReferenceId),
// 		zap.Any("attributes", req.Attributes),
// 	}

// 	zap.L().With(fields...).Info("Incoming Request")

// 	info := workflow.GetInfo(ctx)

// 	ao := workflow.ActivityOptions{
// 		StartToCloseTimeout: 30 * time.Second,
// 		RetryPolicy: &temporal.RetryPolicy{
// 			InitialInterval:    time.Second,
// 			BackoffCoefficient: 2.0,
// 			MaximumAttempts:    3,
// 		},
// 	}

// 	ctx = workflow.WithActivityOptions(ctx, ao)

// 	var actions []*domain.NodeAction
// 	if err := workflow.ExecuteActivity(ctx, "CallEvaluateRule", req).Get(ctx, &actions); err != nil {
// 		zap.L().With(fields...).Error("failed to ExecuteActivity", zap.Error(err), zap.String("activities", "CallEvaluateRule"))
// 		return err
// 	}

// 	if len(actions) == 0 {
// 		zap.L().With(fields...).Info("action not available")
// 		return fmt.Errorf("action not available")
// 	}

// 	transaction := req.GetTransaction()
// 	for _, action := range actions {
// 		switch workflowv1.ActionType(workflowv1.ActionType_value[action.Type]) {
// 		case workflowv1.ActionType_REWARD_POINT:

// 			b, err := action.Parameters.MarshalJSON()
// 			if err != nil {
// 				return err
// 			}

// 			var param workflowv1.RewardPoint
// 			if err := json.Unmarshal(b, &param); err != nil {
// 				return err
// 			}

// 			point := ValidationRewardPointType(&param, transaction)
// 			if point == 0 {
// 				return fmt.Errorf("reward point calculation result is zero: rule ineligible or invalid configuration")
// 			}

// 			childOpts := workflow.ChildWorkflowOptions{
// 				WorkflowID:        fmt.Sprintf("%s-%s", info.WorkflowExecution.ID, uuid.NewString()),
// 				ParentClosePolicy: enums.PARENT_CLOSE_POLICY_ABANDON,
// 			}

// 			// Create a flow for the reward point
// 			b, _ = json.Marshal(req)
// 			var metadata map[string]string
// 			if err := json.Unmarshal(b, &metadata); err != nil {
// 				return err
// 			}

// 			ctxChild := workflow.WithChildOptions(ctx, childOpts)
// 			if err := workflow.ExecuteChildWorkflow(ctx, WorkflowLedgerEntry, &ledgerv1.AddEntryRequest{
// 				OrganizationId: req.OrganizationId,
// 				UserId:         req.UserId,
// 				ReferenceId:    req.ReferenceId,
// 				Type:           ledgerv1.EntryType_CREDIT,
// 				Amount:         point,
// 				Metadata:       metadata,
// 			}).Get(ctxChild, nil); err != nil {
// 				return err
// 			}

// 		}
// 	}

// 	return nil
// }

// func ValidationRewardPointType(p *workflowv1.RewardPoint, input *pointv1.TransactionAttributes) int64 {
// 	switch p.RewardPointType {
// 	case workflowv1.RewardPointType_MULTIPLE:

// 		unitAmount := p.UnitAmount
// 		amount := input.Amount
// 		rewardValue := p.RewardPointValue

// 		units := math.Floor(float64(amount) / float64(unitAmount))
// 		pointsPerUnit := math.Floor(float64(unitAmount) * rewardValue)

// 		return int64(units * pointsPerUnit)
// 	case workflowv1.RewardPointType_PERCENT_BPS:
// 		return 0
// 	case workflowv1.RewardPointType_PER_UNIT:
// 		return 0
// 	default:
// 		return int64(p.RewardPointValue)
// 	}
// }

// func LedgerEntry(ctx workflow.Context, req *ledgerv1.AddEntryRequest) (*ledgerv1.LedgerEntry, error) {
// 	fields := []zap.Field{
// 		zap.String("organization_id", req.OrganizationId),
// 		zap.String("user_id", req.UserId),
// 		zap.String("reference_id", req.ReferenceId),
// 		zap.String("type", req.Type.String()),
// 		zap.Int64("amount", req.Amount),
// 		zap.Any("metadata", req.Metadata),
// 	}

// 	ao := workflow.ActivityOptions{
// 		StartToCloseTimeout: 30 * time.Second,
// 		RetryPolicy: &temporal.RetryPolicy{
// 			InitialInterval:    time.Second,
// 			BackoffCoefficient: 2.0,
// 			MaximumAttempts:    3,
// 		},
// 	}

// 	ctx = workflow.WithActivityOptions(ctx, ao)

// 	var entry *ledgerv1.LedgerEntry
// 	if err := workflow.ExecuteActivity(ctx, activities.CreateLedgerEntry, req).Get(ctx, &entry); err != nil {
// 		zap.L().With(fields...).Error("failed to ExecuteActivity", zap.Error(err), zap.String("activities", activities.CreateLedgerEntry))
// 		return nil, err
// 	}

// 	return entry, nil
// }
