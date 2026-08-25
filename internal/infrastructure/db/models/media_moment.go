package models

import (
	"encoding/json"
	"time"
)

type MediaAsset struct {
	ID               string `gorm:"type:uuid;primaryKey"`
	OwnerUserID      uint64 `gorm:"not null;index"`
	Provider         string
	Bucket           string
	StagingObjectKey string `gorm:"not null;uniqueIndex"`
	StagingVersionID *string
	FinalObjectKey   string `gorm:"not null;uniqueIndex"`
	FinalVersionID   *string
	ContentType      string
	Bytes            int64
	ChecksumSHA256   string    `gorm:"size:44;not null"`
	State            string    `gorm:"not null;index"`
	UploadExpiresAt  time.Time `gorm:"not null;index"`
	RetentionDueAt   time.Time `gorm:"not null;index"`
	AvailableAt      *time.Time
	FailedAt         *time.Time
	DeletedAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (*MediaAsset) TableName() string { return "media_assets" }

type MediaProcessingClaim struct {
	MediaAssetID      string `gorm:"type:uuid;primaryKey"`
	ClaimToken        string
	OperationKey      string
	Stage             string
	StagingVersionID  *string
	FinalVersionID    *string
	LeaseExpiresAt    time.Time `gorm:"not null;index"`
	AttemptCount      int
	LastErrorCategory *string
	CompletedAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (*MediaProcessingClaim) TableName() string { return "media_processing_claims" }

type IdempotencyOperation struct {
	ID               string `gorm:"type:uuid;primaryKey"`
	ActorUserID      uint64
	IdempotencyKey   string `gorm:"type:uuid;not null"`
	Operation        string
	ResourceRef      *string
	IntentHash       string
	State            string
	ResultRef        *string
	ResultBoolean    *bool
	ResultCount      *int
	ResponseSnapshot json.RawMessage `gorm:"type:jsonb;not null"`
	HTTPStatus       int
	CreatedAt        time.Time
	CompletedAt      *time.Time
}

func (*IdempotencyOperation) TableName() string { return "idempotency_operations" }

type MediaCleanupJob struct {
	ID             string `gorm:"type:uuid;primaryKey"`
	MediaAssetID   string `gorm:"type:uuid;not null;index"`
	Kind           string
	State          string
	DueAt          time.Time
	AttemptCount   int
	MaxAttempts    int
	NextAttemptAt  time.Time
	ClaimToken     *string
	LeaseExpiresAt *time.Time
	LastErrorCode  *string
	CompletedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (*MediaCleanupJob) TableName() string { return "media_cleanup_jobs" }

type Moment struct {
	ID                string `gorm:"type:uuid;primaryKey"`
	UserID            uint64
	ParticipationID   *string
	ActivityID        *string
	MediaAssetID      string
	Origin            string
	PublicationStatus string
	ModerationStatus  string
	RewardStatus      string
	PointsAwarded     int
	CapturedAt        time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (*Moment) TableName() string { return "moments" }

type MomentLike struct {
	MomentID  string
	UserID    uint64
	CreatedAt time.Time
}

func (*MomentLike) TableName() string { return "moment_likes" }

type MomentModerationDecision struct {
	ID             string
	MomentID       string
	ActorUserID    uint64
	Action         string
	IdempotencyKey string
	CreatedAt      time.Time
}

func (*MomentModerationDecision) TableName() string { return "moment_moderation_decisions" }
