package task

import (
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
)

const TypeTransactionCompleted = "transaction:completed"

type TransactionEventPayload struct {
	TenantID      string                 `json:"tenant_id"`
	TransactionID string                 `json:"transaction_id"`
	UserID        string                 `json:"user_id"`
	Amount        float64                `json:"amount"`
	Currency      string                 `json:"currency"`
	Channel       string                 `json:"channel"`
	Status        string                 `json:"status"`
	Metadata      map[string]interface{} `json:"metadata"`
	CreatedAt     time.Time              `json:"created_at"`
}

func NewTransactionCompletedTask(p TransactionEventPayload) (*asynq.Task, error) {
	payload, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeTransactionCompleted, payload,
		asynq.Queue("transaction-events")), nil
}
