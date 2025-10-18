package taskname

const (
	// Tenant task
	TenantProvisioningLoyalty = "tenant:provisioning:loyalty"

	// Transaction task
	TransactionCompleted = "transaction:completed"

	// Loyalty tasks
	LoyaltyExpiryRun  = "loyalty:expiry:run"
	LoyaltySyncLedger = "loyalty:sync:ledger"

	// Voucher tasks
	VoucherCleanupExpired = "voucher:cleanup:expired"

	// Tenant tasks
	TenantBillingSync = "tenant:billing:sync"
)
