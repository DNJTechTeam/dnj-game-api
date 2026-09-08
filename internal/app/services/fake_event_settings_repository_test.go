package services

import (
	"context"
	"time"

	entities "github.com/dnjtechteam/dnj-game-api/internal/domain/eventsettings"
)

type fakeEventSettingsRepository struct {
	closed bool
	err    error
}

func newFakeEventSettingsRepository() *fakeEventSettingsRepository {
	return &fakeEventSettingsRepository{closed: false}
}

func (r *fakeEventSettingsRepository) Get(ctx context.Context) (*entities.EventSettings, error) {
	if r.err != nil {
		return nil, r.err
	}
	now := time.Now().UTC()
	return &entities.EventSettings{
		ID:            "global",
		ScoringClosed: r.closed,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (r *fakeEventSettingsRepository) SetScoringClosed(ctx context.Context, closed bool, closedBy *uint64, auditNote string) error {
	r.closed = closed
	return nil
}
