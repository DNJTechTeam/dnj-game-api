package entities

import "time"

type GroupMembership struct {
	ID        uint64
	UserID    uint64
	GroupID   uint64
	JoinedAt  time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type GroupMember struct {
	MembershipID uint64
	UserID       uint64
	Name         string
	Role         string
	JoinedAt     time.Time
}
