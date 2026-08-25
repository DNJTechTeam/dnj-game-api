package entities

import (
	"errors"
	"time"
)

var (
	ErrObjectNotFound      = errors.New("storage object not found")
	ErrProviderUnavailable = errors.New("storage provider unavailable")
	ErrObjectTooLarge      = errors.New("storage object exceeds limit")
)

type AssetState string

const (
	AssetPendingUpload AssetState = "pending_upload"
	AssetProcessing    AssetState = "processing"
	AssetAvailable     AssetState = "available"
	AssetFailed        AssetState = "failed"
	AssetDeleted       AssetState = "deleted"
)

type Asset struct {
	ID               string
	OwnerUserID      uint64
	Provider         string
	Bucket           string
	StagingObjectKey string
	StagingVersionID *string
	FinalObjectKey   string
	FinalVersionID   *string
	ContentType      string
	Bytes            int64
	ChecksumSHA256   string
	State            AssetState
	UploadExpiresAt  time.Time
	RetentionDueAt   time.Time
	AvailableAt      *time.Time
	FailedAt         *time.Time
	DeletedAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ProcessingClaim struct {
	MediaAssetID      string
	ClaimToken        string
	OperationKey      string
	Stage             string
	StagingVersionID  *string
	FinalVersionID    *string
	LeaseExpiresAt    time.Time
	AttemptCount      int
	LastErrorCategory *string
	CompletedAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Operation struct {
	ID               string
	ActorUserID      uint64
	IdempotencyKey   string
	Operation        string
	ResourceRef      *string
	IntentHash       string
	State            string
	ResultRef        *string
	ResultBoolean    *bool
	ResultCount      *int
	ResponseSnapshot []byte
	HTTPStatus       int
	CreatedAt        time.Time
	CompletedAt      *time.Time
}

type CleanupJob struct {
	ID             string
	MediaAssetID   string
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
type ObjectMetadata struct {
	ContentType    string
	Bytes          int64
	ChecksumSHA256 string
	VersionID      string
}
type ObjectBody struct {
	Metadata ObjectMetadata
	Bytes    []byte
}
type PresignedRequest struct {
	URL       string
	Method    string
	Headers   map[string]string
	ExpiresAt time.Time
}
type WorkerMetrics struct {
	Pending             int64
	Processing          int64
	Expired             int64
	Failed              int64
	Retries             int64
	OldestJobAgeSeconds float64
}
