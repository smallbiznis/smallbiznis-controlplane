package taskname

const (
	// Tenant task
	TenantProvisioningLoyalty = "tenant:provisioning:loyalty"

	// Transaction task
	TransactionCreated  = "transaction:created"
	TransactionRefunded = "transaction:refunded"

	// Loyalty tasks
	LoyaltyEarning    = "loyalty:earning"
	LoyaltyExpiryRun  = "loyalty:expiry:run"
	LoyaltySyncLedger = "loyalty:sync:ledger"

	// Voucher tasks
	VoucherCleanupExpired = "voucher:cleanup:expired"

	// Tenant tasks
	TenantBillingSync = "tenant:billing:sync"
)
