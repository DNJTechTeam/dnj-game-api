package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
)

type GameServiceInterface interface {
	ListGames(ctx context.Context, filter *messages.ListGamesFilterDTO) (*messages.PaginatedResponse[messages.GameResponseDTO], error)
	GetGame(ctx context.Context, gameID string) (*messages.GameResponseDTO, error)
	Rankings(ctx context.Context, scope string, page uint64) (*messages.RankingResponseDTO, error)
	Overview(ctx context.Context) (*messages.GameOverviewResponseDTO, error)
	CurrentRun(ctx context.Context, runID string) (*messages.ParticipantRunEnvelopeDTO, error)
	CurrentParticipation(ctx context.Context) (*messages.ParticipationEnvelopeDTO, error)
	ValidateQR(ctx context.Context, request *messages.QRValidateRequestDTO) (*messages.ParticipationEnvelopeDTO, int, error)

	ManagerOverview(ctx context.Context) (*messages.ManagerGameOverviewResponseDTO, error)
	ManagerRun(ctx context.Context, runID string) (*messages.ManagerRunResponseDTO, error)
	CreateRun(ctx context.Context, key string, request *messages.CreateRunRequestDTO) (*messages.ManagerRunResponseDTO, int, error)
	RotateQR(ctx context.Context, runID, key string) (*messages.QRResponseDTO, int, error)
	StartRun(ctx context.Context, runID, key string) (*messages.ManagerRunResponseDTO, error)
	PauseRun(ctx context.Context, runID, key string) (*messages.ManagerRunResponseDTO, error)
	ResumeRun(ctx context.Context, runID, key string) (*messages.ManagerRunResponseDTO, error)
	FinalizeRun(ctx context.Context, runID, key string, request *messages.FinalizeRunResultsRequestDTO) (*messages.ManagerRunResponseDTO, error)
	CancelRun(ctx context.Context, runID, key string) (*messages.ManagerRunResponseDTO, error)
}
