package models

import "time"

type Activity struct {
	ID              string     `gorm:"type:uuid;primaryKey"`
	SpaceID         *string    `gorm:"type:uuid;index;default:null"`
	Slug            string     `gorm:"not null;uniqueIndex"`
	Name            string     `gorm:"not null"`
	Description     *string    `gorm:"default:null"`
	Kind            string     `gorm:"not null"`
	Status          string     `gorm:"not null;default:'draft'"`
	StartsAt        *time.Time `gorm:"default:null"`
	EndsAt          *time.Time `gorm:"default:null"`
	CheckInPoints   int        `gorm:"not null;default:0"`
	MomentPoints    int        `gorm:"not null;default:0"`
	CooldownSeconds int        `gorm:"not null;default:0"`
	AllowsMoment    bool       `gorm:"not null;default:false"`
	CreatedAt       time.Time  `gorm:"autoCreateTime:nano"`
	UpdatedAt       time.Time  `gorm:"autoUpdateTime:nano"`
}

func (*Activity) TableName() string { return "activities" }

type ActivityManagerAssignment struct {
	ActivityID string    `gorm:"type:uuid;primaryKey"`
	UserID     uint64    `gorm:"primaryKey"`
	CreatedAt  time.Time `gorm:"autoCreateTime:nano"`
}

func (*ActivityManagerAssignment) TableName() string { return "activity_manager_assignments" }
