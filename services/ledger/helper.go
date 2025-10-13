package ledger

type RedeemAllocation struct {
	CreditPoolID    string
	SourceID        string
	Amount          int64
	RemainingAmount int64
}

type MetaDebit struct {
	LedgerEntryID string `json:"ledger_entry_id"`
	Amount        int64  `json:"amount"`
}
