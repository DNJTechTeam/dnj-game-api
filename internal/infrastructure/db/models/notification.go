package models

import (
	"encoding/json"
	"time"
)

// PointsEnabled and AnnouncementEnabled intentionally carry no gorm "default"
// tag: the service always supplies an explicit value on write, and GORM
// treats a Go zero value (false) on a "default"-tagged field as "unset",
// silently substituting the column default instead of persisting false.
type NotificationPreference struct {
	UserID              uint64 `gorm:"primaryKey"`
	PointsEnabled       bool   `gorm:"not null"`
	AnnouncementEnabled bool   `gorm:"not null"`
	UpdatedAt           time.Time
}

func (*NotificationPreference) TableName() string { return "notification_preferences" }

type Notification struct {
	ID         string `gorm:"type:uuid;primaryKey"`
	UserID     uint64
	Category   string
	State      string
	Title      string
	Body       string
	SourceType string
	SourceID   *string
	Metadata   json.RawMessage
	CreatedAt  time.Time
	ReadAt     *time.Time
}

func (*Notification) TableName() string { return "notifications" }
