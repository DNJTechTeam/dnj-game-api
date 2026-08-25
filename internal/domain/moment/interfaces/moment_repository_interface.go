package interfaces

import (
	"context"
	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/moment/entities"
	"time"
)

type Repository interface {
	FindParticipationForUpdate(context.Context, string) (*gameEntities.Participation, error)
	FindActivityForUpdate(context.Context, string) (string, bool, *time.Time, *time.Time, int, string, *string, error)
	CreateMoment(context.Context, *entities.Moment) error
	FindMoment(context.Context, string, uint64, bool) (*entities.Moment, error)
	ListMoments(context.Context, string, uint64, *uint64, *entities.Cursor, time.Time) (*entities.Page, error)
	ToggleLike(context.Context, string, uint64, time.Time) (bool, int, error)
	ListModeration(context.Context, string, uint64, time.Time) (*entities.ModerationPage, error)
	ApplyModeration(
		context.Context,
		string,
		string,
		uint64,
		string,
		time.Time,
	) (*entities.Moment, *mediaEntities.Asset, bool, error)
	CreateModerationDecision(context.Context, *entities.ModerationDecision) (bool, error)
	AwardMoment(context.Context, string, uint64, string, int, time.Time) error
	ReverseMomentAward(context.Context, string, uint64, time.Time) (bool, error)
}
