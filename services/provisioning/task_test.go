package provisioning

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewTenantProvisionTasks(t *testing.T) {
	payload := TenantProvisionPayload{
		TenantID: 42,
		Slug:     "demo",
		Name:     "Demo Tenant",
		Domain:   "demo.example.com",
	}

	tasks := NewTenantProvisionTasks(payload)

	require.Len(t, tasks, 3)
	expectedTypes := []string{TaskTenantProvisionLoyalty, TaskTenantProvisionVoucher, TaskTenantPostSetup}
	for i, task := range tasks {
		require.Equal(t, expectedTypes[i], task.Type())

		var decoded TenantProvisionPayload
		require.NoError(t, json.Unmarshal(task.Payload(), &decoded))
		require.Equal(t, payload, decoded)
	}
}
