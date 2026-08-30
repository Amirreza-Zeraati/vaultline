package models

import "github.com/google/uuid"

const (
	DirectionDebit  = "debit"
	DirectionCredit = "credit"
)

var ValidDirection = []string{DirectionDebit, DirectionCredit}

func IsValidDirection(direction string) bool {
	for _, d := range ValidDirection {
		if d == direction {
			return true
		}
	}
	return false
}

type LedgerEntry struct {
	Base
	TransferID   uuid.UUID `gorm:"type:uuid;not null;index" json:"transfer_id"`
	AccountID    uuid.UUID `gorm:"type:uuid;not null;index" json:"account_id"`
	Amount       int64     `gorm:"not null" json:"amount"`        // negative = debit, positive = credit
	Direction    string    `gorm:"not null" json:"direction"`     // DirectionDebit or DirectionCredit
	BalanceAfter int64     `gorm:"not null" json:"balance_after"` // account's balance snapshot right after this entry
}

func (LedgerEntry) TableName() string { return "ledger_entries" }
