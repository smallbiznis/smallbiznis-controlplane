package provisioning

import (
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
)

const (
	TaskTenantProvisionLoyalty = "tenant:provision:loyalty"
	TaskTenantProvisionVoucher = "tenant:provision:voucher"
	TaskTenantPostSetup        = "tenant:post-setup"
)

type TenantProvisionPayload struct {
	TenantID int64  `json:"tenant_id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Domain   string `json:"domain"`
}

func NewTenantProvisionTasks(p TenantProvisionPayload) []*asynq.Task {
	payload, _ := json.Marshal(p)
	return []*asynq.Task{
		asynq.NewTask(TaskTenantProvisionLoyalty, payload,
			asynq.MaxRetry(3),
			asynq.Timeout(60*time.Second),
			asynq.Queue("provisioning")),
		asynq.NewTask(TaskTenantProvisionVoucher, payload,
			asynq.MaxRetry(3),
			asynq.Queue("provisioning")),
		asynq.NewTask(TaskTenantPostSetup, payload,
			asynq.Queue("provisioning")),
	}
}
