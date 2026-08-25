package interfaces

import (
	"context"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
)

type Repository interface {
	CreateAsset(context.Context, *entities.Asset) error
	FindAsset(context.Context, string, bool) (*entities.Asset, error)
	UpdateAsset(context.Context, *entities.Asset) error
	FindOperation(context.Context, uint64, string) (*entities.Operation, error)
	CreateOperation(context.Context, *entities.Operation) error
	CompleteOperation(context.Context, string, int, *string, *bool, *int, []byte, time.Time) error
	FindLegacyOperation(context.Context, uint64, string) (bool, error)
	AcquireProcessingClaim(
		context.Context,
		string,
		string,
		string,
		time.Time,
		time.Time,
	) (*entities.ProcessingClaim, bool, error)
	UpdateProcessingClaim(context.Context, *entities.ProcessingClaim) error
	CreateCleanupJob(context.Context, *entities.CleanupJob) (bool, error)
	ClaimCleanupJobs(context.Context, time.Time, time.Duration, int) ([]entities.CleanupJob, error)
	CompleteCleanupJob(context.Context, string, string, time.Time) error
	RetryCleanupJob(context.Context, string, string, time.Time, string, time.Time) error
	ExpirePendingAssets(context.Context, time.Time, int) ([]entities.Asset, error)
	Metrics(context.Context, time.Time) (*entities.WorkerMetrics, error)
}
type Storage interface {
	ValidateConfiguration() error
	PresignUpload(context.Context, *entities.Asset, time.Time) (*entities.PresignedRequest, error)
	PresignDownload(context.Context, *entities.Asset, time.Time, time.Duration) (string, error)
	HeadStaging(context.Context, *entities.Asset) (*entities.ObjectMetadata, error)
	DownloadStaging(context.Context, *entities.Asset, string, int64) (*entities.ObjectBody, error)
	PutFinal(context.Context, *entities.Asset, []byte, string) (*entities.ObjectMetadata, error)
	HeadFinal(context.Context, *entities.Asset, string) (*entities.ObjectMetadata, error)
	DeleteObjectVersions(context.Context, string, string) error
}
