package models

import (
	"encoding/json"
	"time"
)

type ActivityRun struct {
	ID         string          `gorm:"type:uuid;primaryKey"`
	ActivityID string          `gorm:"type:uuid;not null;index"`
	StartedBy  uint64          `gorm:"not null;index"`
	Status     string          `gorm:"not null;index"`
	PointRules json.RawMessage `gorm:"type:jsonb;not null"`
	StartedAt  *time.Time      `gorm:"default:null"`
	EndedAt    *time.Time      `gorm:"default:null"`
	CreatedAt  time.Time       `gorm:"autoCreateTime:nano"`
	UpdatedAt  time.Time       `gorm:"autoUpdateTime:nano"`
}

func (*ActivityRun) TableName() string { return "activity_runs" }

type Participation struct {
	ID             string    `gorm:"type:uuid;primaryKey"`
	UserID         uint64    `gorm:"not null;index"`
	ActivityID     string    `gorm:"type:uuid;not null;index"`
	ActivityRunID  string    `gorm:"type:uuid;not null;index"`
	QRCodeID       string    `gorm:"type:uuid;not null"`
	CheckedInAt    time.Time `gorm:"not null;index"`
	Status         string    `gorm:"not null;index"`
	CanShareMoment bool      `gorm:"not null;default:false"`
	CheckInPoints  int       `gorm:"not null;default:0"`
	CreatedAt      time.Time `gorm:"autoCreateTime:nano"`
}

func (*Participation) TableName() string { return "participations" }

type ActivityRunParticipant struct {
	ID              string    `gorm:"type:uuid;primaryKey"`
	ActivityRunID   string    `gorm:"type:uuid;not null;index"`
	UserID          uint64    `gorm:"not null;index"`
	ParticipationID string    `gorm:"type:uuid;not null;uniqueIndex"`
	CheckedInAt     time.Time `gorm:"not null"`
	Result          *string   `gorm:"default:null"`
	PointsAwarded   int       `gorm:"not null;default:0"`
	CreatedAt       time.Time `gorm:"autoCreateTime:nano"`
}

func (*ActivityRunParticipant) TableName() string { return "activity_run_participants" }

type ActivityRunQRCode struct {
	ID            string    `gorm:"type:uuid;primaryKey"`
	ActivityID    string    `gorm:"type:uuid;not null;index"`
	ActivityRunID string    `gorm:"type:uuid;not null;index"`
	TokenHash     string    `gorm:"size:64;not null;uniqueIndex"`
	ExpiresAt     time.Time `gorm:"not null;index"`
	Status        string    `gorm:"not null;index"`
	CreatedAt     time.Time `gorm:"autoCreateTime:nano"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime:nano"`
}

func (*ActivityRunQRCode) TableName() string { return "activity_run_qr_codes" }

type PointEntry struct {
	ID              string    `gorm:"type:uuid;primaryKey"`
	UserID          uint64    `gorm:"not null;index"`
	ActivityID      *string   `gorm:"type:uuid;index;default:null"`
	ActivityRunID   *string   `gorm:"type:uuid;index;default:null"`
	ParticipationID *string   `gorm:"type:uuid;index;default:null"`
	MomentID        *string   `gorm:"type:uuid;index;default:null"`
	Origin          string    `gorm:"not null"`
	Reason          string    `gorm:"not null"`
	Delta           int       `gorm:"not null"`
	CreatedAt       time.Time `gorm:"autoCreateTime:nano;index"`
}

func (*PointEntry) TableName() string { return "point_entries" }

type ManagerOperation struct {
	ID              string     `gorm:"type:uuid;primaryKey"`
	ActorUserID     uint64     `gorm:"not null;index"`
	IdempotencyKey  string     `gorm:"type:uuid;not null"`
	Operation       string     `gorm:"not null"`
	ActivityID      string     `gorm:"type:uuid;not null;index"`
	ActivityRunID   *string    `gorm:"type:uuid;index;default:null"`
	IntentHash      string     `gorm:"size:64;not null"`
	ResultRef       *string    `gorm:"type:uuid;default:null"`
	ResultStatus    *string    `gorm:"default:null"`
	ResultStartedAt *time.Time `gorm:"default:null"`
	ResultEndedAt   *time.Time `gorm:"default:null"`
	ResultExpiresAt *time.Time `gorm:"default:null"`
	HTTPStatus      int        `gorm:"not null"`
	CreatedAt       time.Time  `gorm:"autoCreateTime:nano"`
}

func (*ManagerOperation) TableName() string { return "manager_operations" }
