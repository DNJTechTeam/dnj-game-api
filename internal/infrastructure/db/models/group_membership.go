package models

import "time"

// GroupMembership is the current, authoritative group relationship for a user.
// users.group_id remains mirrored while V1 consumers are supported.
type GroupMembership struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	UserID    uint64    `gorm:"not null;uniqueIndex"`
	GroupID   uint64    `gorm:"not null;index"`
	JoinedAt  time.Time `gorm:"not null"`
	CreatedAt time.Time `gorm:"autoCreateTime:nano"`
	UpdatedAt time.Time `gorm:"autoUpdateTime:nano"`
}

func (GroupMembership) TableName() string { return "group_memberships" }
