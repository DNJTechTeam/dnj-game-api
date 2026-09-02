package models

import (
	"encoding/json"
	"time"
)

type SpecialEvent struct {
	ID              string          `gorm:"type:uuid;primaryKey"`
	ActivityID      string          `gorm:"type:uuid;not null;uniqueIndex"`
	ActivityRunID   *string         `gorm:"type:uuid;index;default:null"`
	Title           string          `gorm:"not null"`
	Description     *string         `gorm:"default:null"`
	Points          int             `gorm:"not null;default:0"`
	DurationMinutes int             `gorm:"not null"`
	Targets         json.RawMessage `gorm:"type:jsonb;not null"`
	Status          string          `gorm:"not null;index"`
	TeaserAt        *time.Time      `gorm:"default:null"`
	EndsAt          time.Time       `gorm:"not null;index"`
	QRToken         *string         `gorm:"type:text;default:null"`
	QRExpiresAt     *time.Time      `gorm:"default:null"`
	CreatedBy       uint64          `gorm:"not null;index"`
	CreatedAt       time.Time       `gorm:"autoCreateTime:nano"`
	UpdatedAt       time.Time       `gorm:"autoUpdateTime:nano"`
}

func (*SpecialEvent) TableName() string { return "special_events" }
