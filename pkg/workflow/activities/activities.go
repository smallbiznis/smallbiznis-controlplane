package activities

const (
	CallEvaluateRule  = "CallEvaluateRule"
	CreateLedgerEntry = "CreateLedgerEntry"
)

// type Activities struct {
// 	Ledger ledgerv1.LedgerServiceClient
// 	Flow   repository.Repository[domain.Flow]
// }

// func (a *Activities) CallEvaluateRule(ctx context.Context, req *pointv1.EarningRequest) ([]*domain.NodeAction, error) {
// 	flow, err := a.Flow.FindOne(ctx, &domain.Flow{
// 		OrganizationID: req.OrganizationId,
// 		Trigger:        workflowv1.TriggerType_TRANSACTION.String(),
// 		Status:         workflowv1.FlowStatus_ACTIVE.String(),
// 	})
// 	if err != nil {
// 		return nil, err
// 	}

// 	if flow == nil {
// 		return nil, fmt.Errorf("workflow not found")
// 	}

// 	attr := celengine.StructToMap(req.GetTransaction())
// 	graph, err := flow.BuildFlowGraph()
// 	if err != nil {
// 		return nil, status.Error(codes.Internal, err.Error())
// 	}

// 	triggerNode := FindTriggerNode(graph.Nodes)
// 	actions, err := ExecuteFlow(ctx, graph, triggerNode, attr)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return actions, nil
// }

// func FindTriggerNode(nodes map[string]*domain.Node) *domain.Node {
// 	for _, node := range nodes {
// 		if node.Type == workflowv1.NodeType_TRIGGER {
// 			return node
// 		}
// 	}
// 	return nil
// }

// func ExecuteFlow(ctx context.Context, graph *domain.FlowGraph, currentNode *domain.Node, input map[string]interface{}) ([]*domain.NodeAction, error) {
// 	var actions []*domain.NodeAction
// 	switch currentNode.Type {
// 	case workflowv1.NodeType_TRIGGER:
// 		pass, err := EvaluateCEL(currentNode.Condition.Expression, input)
// 		if err != nil {
// 			zap.L().Error("failed evaluate cel", zap.Error(err))
// 			return nil, err
// 		}

// 		edges := graph.Edges[currentNode.ID]
// 		for _, edge := range edges {
// 			if (pass && edge.Type == workflowv1.EdgeType_EDGE_TYPE_THEN.String()) || (!pass && edge.Type == workflowv1.EdgeType_EDGE_TYPE_OTHERWISE.String()) {
// 				return ExecuteFlow(ctx, graph, graph.Nodes[edge.Target], input)
// 			}
// 		}

// 	case workflowv1.NodeType_ACTION:
// 		actions = append(actions, currentNode.Action)
// 	case workflowv1.NodeType_CONDITION:
// 		edges := graph.Edges[currentNode.ID]
// 		for _, edge := range edges {
// 			return ExecuteFlow(ctx, graph, graph.Nodes[edge.Target], input)
// 		}
// 	}

// 	return actions, nil
// }

// func EvaluateCEL(expr string, attrs map[string]interface{}) (bool, error) {
// 	env, err := celengine.BuildCelEnvFromAttributes(attrs)
// 	if err != nil {
// 		return false, status.Errorf(codes.Internal, "failed to BuildCelEnvFromAttributes: %v", err)
// 	}

// 	ast, issues := env.Compile(expr)
// 	if issues != nil && issues.Err() != nil {
// 		return false, issues.Err()
// 	}

// 	prg, err := env.Program(ast)
// 	if err != nil {
// 		return false, err
// 	}

// 	out, _, err := prg.Eval(attrs)
// 	if err != nil {
// 		return false, err
// 	}

// 	return out.Value().(bool), nil
// }

// func (a *Activities) CreateLedgerEntry(ctx context.Context, req *ledgerv1.AddEntryRequest) (*ledgerv1.LedgerEntry, error) {
// 	return a.Ledger.AddEntry(ctx, req)
// }

// func (a *Activities) Reward(ctx context.Context) {}
