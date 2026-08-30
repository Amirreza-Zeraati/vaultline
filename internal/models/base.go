package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Base struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func GenerateModelID(id *uuid.UUID) error {
	if *id == uuid.Nil {
		newUUID, err := uuid.NewV7()
		if err != nil {
			return err
		}
		*id = newUUID
	}
	return nil
}

func (b *Base) BeforeCreate(_ *gorm.DB) error {
	return GenerateModelID(&b.ID)
}
