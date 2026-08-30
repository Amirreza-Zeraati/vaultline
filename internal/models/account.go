package models

import "github.com/google/uuid"

type Account struct {
	Base
	OwnerID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"owner_id"`
	Currency string    `gorm:"not null;default:USD" json:"currency"`
	Balance  int64     `gorm:"not null;default:0" json:"balance"`
	Version  int64     `gorm:"not null;default:0" json:"-"`
}

func (Account) TableName() string { return "accounts" }
