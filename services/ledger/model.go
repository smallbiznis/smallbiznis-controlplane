package ledger

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/datatypes"
)

type Balance struct {
	ID        string    `gorm:"column:id"`
	TenantID  string    `gorm:"column:tenant_id"`
	MemberID  string    `gorm:"column:member_id"`
	Balance   int64     `gorm:"column:balance"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

type CreditPool struct {
	ID            string    `gorm:"column:id"`
	LedgerEntryID string    `gorm:"column:ledger_entry_id"`
	TenantID      string    `gorm:"column:tenant_id"`
	MemberID      string    `gorm:"column:member_id"`
	Remaining     int64     `gorm:"column:remaining"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

type LedgerEntry struct {
	ID            string         `gorm:"column:id"`
	CreatedAt     time.Time      `gorm:"column:created_at"`
	UpdatedAt     time.Time      `gorm:"column:updated_at"`
	TenantID      string         `gorm:"column:tenant_id"`
	MemberID      string         `gorm:"column:member_id"`
	Type          string         `gorm:"column:type"`
	Amount        int64          `gorm:"column:amount"`
	TransactionID string         `gorm:"column:transaction_id"`
	ReferenceID   string         `gorm:"column:reference_id"`
	Description   string         `gorm:"column:description"`
	PreviousHash  string         `gorm:"column:previous_hash"`
	Hash          string         `gorm:"column:hash"`
	Metadata      datatypes.JSON `gorm:"column:metadata"`
}

type LedgerParams struct {
	LedgerID      string
	TenantID      string
	MemberID      string
	Type          string
	Amount        int64
	ReferenceID   string
	TransactionID string
	Description   string
	PreviousHash  string
	Metadata      datatypes.JSON
}

func NewLedgerEntry(p LedgerParams) *LedgerEntry {
	return &LedgerEntry{
		ID:            p.LedgerID,
		TenantID:      p.TenantID,
		MemberID:      p.MemberID,
		Type:          p.Type,
		Amount:        p.Amount,
		TransactionID: p.TransactionID,
		ReferenceID:   p.ReferenceID,
		Description:   p.Description,
		PreviousHash:  p.PreviousHash,
		Metadata:      p.Metadata,
	}
}

func (m *LedgerEntry) HashFields() map[string]string {
	return map[string]string{
		"id":             m.ID,
		"tenant_id":      m.TenantID,
		"member_id":      m.MemberID,
		"type":           m.Type,
		"amount":         fmt.Sprintf("%d", m.Amount),
		"transaction_id": m.TransactionID,
		"reference_id":   m.ReferenceID,
		"description":    m.Description,
		"created_at":     m.CreatedAt.UTC().Format(time.RFC3339Nano),
		"previous_hash":  m.PreviousHash,
	}
}

func (l *LedgerEntry) GenerateHash() string {
	fields := l.HashFields()
	var keys []string
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, fields[k]))
	}

	joined := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(hash[:])
}

func GenerateTransactionID() (string, error) {
	datePart := time.Now().Format("20060102") // YYMMDD

	r := make([]byte, 3) // 3 bytes = 6 hex chars
	_, err := rand.Read(r)
	if err != nil {
		return "", err
	}
	randomPart := strings.ToUpper(fmt.Sprintf("%x", r))

	return fmt.Sprintf("%s-%s", datePart, randomPart), nil
}
