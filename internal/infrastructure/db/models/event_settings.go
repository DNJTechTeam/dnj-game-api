package models

import "time"

type EventSettings struct {
	ID             string     `gorm:"type:varchar(32);primaryKey"`
	ScoringClosed  bool       `gorm:"not null;default:false"`
	ClosedAt       *time.Time `gorm:"default:null"`
	ClosedBy       *uint64    `gorm:"default:null"`
	AuditNote      string     `gorm:"type:text;not null;default:''"`
	CreatedAt      time.Time  `gorm:"autoCreateTime:nano"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime:nano"`
}

func (*EventSettings) TableName() string { return "event_settings" }
