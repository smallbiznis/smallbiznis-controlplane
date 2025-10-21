package loyalty

const (
	LoyaltyProcessEarning = "loyalty:process_earning"
)

type ProcessEarningPayload struct {
	EarningID   string `json:"earning_id"`
	TenantID    string `json:"tenant_id"`
	UserID      string `json:"user_id"`
	ReferenceID string `json:"reference_id"`
	EventType   string `json:"event_type,omitempty"`
	TraceID     string `json:"trace_id,omitempty"`
}
