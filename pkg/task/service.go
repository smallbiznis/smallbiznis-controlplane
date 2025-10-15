package task

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
)

type Enqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

type enqueuerImpl struct {
	client *asynq.Client
}

// NewEnqueuer creates a new Enqueuer instance using asynq.Client.
func NewEnqueuer(client *asynq.Client) Enqueuer {
	return &enqueuerImpl{client: client}
}

func (e *enqueuerImpl) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	info, err := e.client.EnqueueContext(context.Background(), task, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue task: %w", err)
	}
	return info, nil
}
