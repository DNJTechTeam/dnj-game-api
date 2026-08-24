package models

import "time"

type Space struct {
	ID           string    `gorm:"type:uuid;primaryKey"`
	Slug         string    `gorm:"not null;uniqueIndex"`
	Name         string    `gorm:"not null"`
	MapReference *string   `gorm:"default:null"`
	CreatedAt    time.Time `gorm:"autoCreateTime:nano"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime:nano"`
}

func (*Space) TableName() string { return "spaces" }
