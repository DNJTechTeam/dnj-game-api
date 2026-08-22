package repositories

import (
	"context"
	"errors"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/groupmembership/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/groupmembership/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GroupMembershipRepository struct {
	*BaseRepository[models.GroupMembership]
}

func NewGroupMembershipRepository(db *gorm.DB) interfaces.GroupMembershipRepositoryInterface {
	return &GroupMembershipRepository{BaseRepository: NewBaseRepository[models.GroupMembership](db)}
}

func (r *GroupMembershipRepository) UpsertForUser(ctx context.Context, membership *entities.GroupMembership) (*entities.GroupMembership, error) {
	model := mappers.MapGroupMembershipEntityToModel(membership)
	err := r.getDB(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"group_id", "joined_at", "updated_at"}),
	}).Create(model).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	var stored models.GroupMembership
	if err := r.getDB(ctx).Where("user_id = ?", membership.UserID).First(&stored).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapGroupMembershipToEntity(&stored), nil
}

func (r *GroupMembershipRepository) DeleteByUser(ctx context.Context, userID uint64) error {
	return handleRepositoryError(r.getDB(ctx).Where("user_id = ?", userID).Delete(&models.GroupMembership{}).Error)
}

func (r *GroupMembershipRepository) FindByUser(ctx context.Context, userID uint64) (*entities.GroupMembership, error) {
	var model models.GroupMembership
	err := r.getDB(ctx).Where("user_id = ?", userID).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, appErrors.ErrNotFound
	}
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapGroupMembershipToEntity(&model), nil
}

func (r *GroupMembershipRepository) ListMembers(ctx context.Context, groupID uint64, page uint64) (*messages.PaginatedResponse[entities.GroupMember], error) {
	const limit = 10
	// Use a concrete scan type because both engines expose timestamptz as time.Time.
	var rows []struct {
		MembershipID uint64
		UserID       uint64
		Name         string
		Role         string
		JoinedAt     time.Time
	}
	err := r.getDB(ctx).Table("group_memberships gm").
		Select("gm.id AS membership_id, gm.user_id, users.name, users.role, gm.joined_at").
		Joins("JOIN users ON users.id = gm.user_id").
		Where("gm.group_id = ?", groupID).
		Order("users.name ASC").Order("gm.user_id ASC").
		Limit(limit + 1).Offset(int(page) * limit).Scan(&rows).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	hasNext := len(rows) > limit
	if hasNext {
		rows = rows[:limit]
	}
	items := make([]entities.GroupMember, len(rows))
	for i := range rows {
		items[i] = entities.GroupMember{MembershipID: rows[i].MembershipID, UserID: rows[i].UserID, Name: rows[i].Name, Role: rows[i].Role, JoinedAt: rows[i].JoinedAt}
	}
	return &messages.PaginatedResponse[entities.GroupMember]{Data: items, Pagination: messages.Pagination{CurrentPage: messages.Uint64StringFromUint64(page + 1), HasNextPage: hasNext, Limit: limit}}, nil
}
