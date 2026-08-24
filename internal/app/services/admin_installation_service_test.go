package services

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pointer[T any](value T) *T { return &value }

func optional[T any](value T) messages.Optional[T] {
	return messages.Optional[T]{Set: true, Value: &value}
}

func nullOptional[T any]() messages.Optional[T] { return messages.Optional[T]{Set: true} }

func setupAdminInstallationTest(t *testing.T) *AdminInstallationService {
	t.Helper()
	TestSuite.DefaultSetup(t)
	for _, model := range []interface{ TableName() string }{
		&models.AdminOperation{}, &models.OperationAudit{}, &models.ActivityManagerAssignment{}, &models.Activity{}, &models.Space{}, &models.User{},
	} {
		TestSuite.TruncateTable(t, model)
	}
	return NewAdminInstallationService(TestSuite.BaseService, TestSuite.SpaceRepository, TestSuite.ActivityRepository, TestSuite.OperationAuditRepository, TestSuite.AdminOperationRepository, TestSuite.UserRepository).(*AdminInstallationService)
}

func seedAdminInstallationUser(t *testing.T, email string, role userEntities.UserRole, onboarding bool) (*userEntities.User, context.Context) {
	t.Helper()
	user, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &userEntities.User{Email: email, Name: email, MobilePhone: "41999999999", Role: role, OnboardingComplete: onboarding})
	require.NoError(t, err)
	return user, TestSuite.ContextWithUser(user.ID)
}

func validCreateSpace(slug string) *messages.CreateAdminSpaceRequestDTO {
	return &messages.CreateAdminSpaceRequestDTO{Slug: pointer(slug), Name: pointer("Espaço " + slug), MapReference: optional("map:" + slug)}
}

func validCreateActivity(slug string, spaceID *string) *messages.CreateAdminActivityRequestDTO {
	startsAt := time.Date(2026, 8, 24, 18, 0, 0, 0, time.UTC)
	endsAt := startsAt.Add(time.Hour)
	request := &messages.CreateAdminActivityRequestDTO{Slug: pointer(slug), Name: pointer("Atividade " + slug), Description: optional("Descrição segura"), Kind: pointer("challenge"), StartsAt: optional(startsAt), EndsAt: optional(endsAt), CheckInPoints: pointer(10), MomentPoints: pointer(20), CooldownSeconds: pointer(60), AllowsMoment: pointer(true)}
	if spaceID != nil {
		request.SpaceID = optional(*spaceID)
	}
	return request
}

func assertAdminError(t *testing.T, err error, status int, code string) {
	t.Helper()
	var apiErr *appErrors.APIServiceError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, status, apiErr.Status)
	assert.Equal(t, code, apiErr.Code)
}

func TestAdminInstallationService_AuthorizationAndDeterministicLists(t *testing.T) {
	// given
	service := setupAdminInstallationTest(t)
	_, adminCtx := seedAdminInstallationUser(t, "admin-list@example.com", userEntities.RoleAdmin, true)
	_, managerCtx := seedAdminInstallationUser(t, "manager-list@example.com", userEntities.RoleEventManager, true)
	_, defaultCtx := seedAdminInstallationUser(t, "default-list@example.com", userEntities.RoleDefault, true)
	for index := 20; index >= 0; index-- {
		id := uuid.NewString()
		now := time.Now().UTC()
		require.NoError(t, TestSuite.DbConn.Create(&models.Space{ID: id, Slug: fmt.Sprintf("space-%02d-%s", index, id), Name: fmt.Sprintf("Space %02d", index), CreatedAt: now, UpdatedAt: now}).Error)
		require.NoError(t, TestSuite.DbConn.Create(&models.Activity{ID: uuid.NewString(), Slug: fmt.Sprintf("activity-%02d-%s", index, id), Name: fmt.Sprintf("Activity %02d", index), Kind: "live", Status: "draft", CreatedAt: now, UpdatedAt: now}).Error)
	}

	// when
	spaces, spacesErr := service.ListSpaces(adminCtx, &messages.ListAdminSpacesFilterDTO{})
	activities, activitiesErr := service.ListActivities(adminCtx, &messages.ListAdminActivitiesFilterDTO{})
	staff, staffErr := service.ListStaff(adminCtx, &messages.ListAdminStaffFilterDTO{Role: "EVENT_MANAGER"})
	_, managerErr := service.ListSpaces(managerCtx, &messages.ListAdminSpacesFilterDTO{})
	_, defaultErr := service.ListActivities(defaultCtx, &messages.ListAdminActivitiesFilterDTO{})
	_, missingIdentityErr := service.ListSpaces(context.Background(), &messages.ListAdminSpacesFilterDTO{})
	_, invalidRoleErr := service.ListStaff(adminCtx, &messages.ListAdminStaffFilterDTO{Role: "ADMIN"})

	// then
	require.NoError(t, spacesErr)
	require.NoError(t, activitiesErr)
	require.NoError(t, staffErr)
	require.Len(t, spaces.Data, 20)
	require.Len(t, activities.Data, 20)
	assert.Equal(t, "Space 00", spaces.Data[0].Name)
	assert.Equal(t, "Activity 00", activities.Data[0].Name)
	assert.True(t, spaces.Pagination.HasNextPage)
	assert.True(t, activities.Pagination.HasNextPage)
	require.Len(t, staff.Data, 1)
	assert.Equal(t, "EVENT_MANAGER", staff.Data[0].Role)
	assertAdminError(t, managerErr, http.StatusForbidden, "FORBIDDEN")
	assertAdminError(t, defaultErr, http.StatusForbidden, "FORBIDDEN")
	assertAdminError(t, missingIdentityErr, http.StatusUnauthorized, "UNAUTHENTICATED")
	assertAdminError(t, invalidRoleErr, http.StatusBadRequest, "INVALID_REQUEST")
}

func TestAdminInstallationService_EveryOperationRequiresDatabaseAdminRole(t *testing.T) {
	// given
	service := setupAdminInstallationTest(t)
	manager, managerCtx := seedAdminInstallationUser(t, "manager-matrix@example.com", userEntities.RoleEventManager, true)
	activityID := uuid.NewString()
	spaceID := uuid.NewString()
	key := uuid.NewString()
	activityRequest := validCreateActivity("forbidden-activity", nil)

	operations := []struct {
		name string
		call func() error
	}{
		{"list spaces", func() error {
			_, err := service.ListSpaces(managerCtx, &messages.ListAdminSpacesFilterDTO{})
			return err
		}},
		{"create space", func() error {
			_, err := service.CreateSpace(managerCtx, key, validCreateSpace("forbidden-space"))
			return err
		}},
		{"update space", func() error {
			_, err := service.UpdateSpace(managerCtx, spaceID, key, &messages.UpdateAdminSpaceRequestDTO{Name: optional("Nome")})
			return err
		}},
		{"list activities", func() error {
			_, err := service.ListActivities(managerCtx, &messages.ListAdminActivitiesFilterDTO{})
			return err
		}},
		{"create activity", func() error { _, err := service.CreateActivity(managerCtx, key, activityRequest); return err }},
		{"update activity", func() error {
			_, err := service.UpdateActivity(managerCtx, activityID, key, &messages.UpdateAdminActivityRequestDTO{Name: optional("Nome")})
			return err
		}},
		{"list staff", func() error {
			_, err := service.ListStaff(managerCtx, &messages.ListAdminStaffFilterDTO{Role: "EVENT_MANAGER"})
			return err
		}},
		{"update role", func() error {
			_, err := service.UpdateUserRole(managerCtx, fmt.Sprint(manager.ID), key, &messages.UpdateAdminUserRoleRequestDTO{Role: pointer("DEFAULT")})
			return err
		}},
		{"list managers", func() error {
			_, err := service.ListManagers(managerCtx, activityID, &messages.ListAdminManagersFilterDTO{})
			return err
		}},
		{"assign manager", func() error {
			_, err := service.AssignManager(managerCtx, activityID, fmt.Sprint(manager.ID), key)
			return err
		}},
		{"remove manager", func() error { return service.RemoveManager(managerCtx, activityID, fmt.Sprint(manager.ID), key) }},
	}

	// when / then
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			assertAdminError(t, operation.call(), http.StatusForbidden, "FORBIDDEN")
		})
	}
}

func TestAdminInstallationService_SpaceWritesAreStrictIdempotentAndAudited(t *testing.T) {
	// given
	service := setupAdminInstallationTest(t)
	_, adminCtx := seedAdminInstallationUser(t, "admin-space@example.com", userEntities.RoleAdmin, true)
	createKey := uuid.NewString()

	// when
	created, createErr := service.CreateSpace(adminCtx, createKey, validCreateSpace("capela"))
	retried, retryErr := service.CreateSpace(adminCtx, createKey, validCreateSpace("capela"))
	updated, updateErr := service.UpdateSpace(adminCtx, created.ID, uuid.NewString(), &messages.UpdateAdminSpaceRequestDTO{Name: optional("Capela Central"), MapReference: nullOptional[string]()})
	originalRetry, originalRetryErr := service.CreateSpace(adminCtx, createKey, validCreateSpace("capela"))
	noOp, noOpErr := service.UpdateSpace(adminCtx, created.ID, uuid.NewString(), &messages.UpdateAdminSpaceRequestDTO{Name: optional("Capela Central")})
	_, reusedErr := service.CreateSpace(adminCtx, createKey, validCreateSpace("auditorio"))
	_, duplicateErr := service.CreateSpace(adminCtx, uuid.NewString(), validCreateSpace("capela"))
	_, malformedKeyErr := service.CreateSpace(adminCtx, "not-a-uuid", validCreateSpace("quadra"))
	_, missingErr := service.UpdateSpace(adminCtx, uuid.NewString(), uuid.NewString(), &messages.UpdateAdminSpaceRequestDTO{Name: optional("Ausente")})
	_, malformedIDErr := service.UpdateSpace(adminCtx, "crossed-user-id", uuid.NewString(), &messages.UpdateAdminSpaceRequestDTO{Name: optional("Ausente")})
	_, emptyErr := service.UpdateSpace(adminCtx, created.ID, uuid.NewString(), &messages.UpdateAdminSpaceRequestDTO{})

	// then
	require.NoError(t, createErr)
	require.NoError(t, retryErr)
	require.NoError(t, updateErr)
	require.NoError(t, originalRetryErr)
	require.NoError(t, noOpErr)
	assert.Equal(t, created, retried)
	assert.Equal(t, created, originalRetry)
	assert.Equal(t, "Capela Central", updated.Name)
	assert.Nil(t, updated.MapReference)
	assert.Equal(t, updated, noOp)
	assertAdminError(t, reusedErr, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")
	assertAdminError(t, duplicateErr, http.StatusConflict, "SLUG_ALREADY_EXISTS")
	assertAdminError(t, malformedKeyErr, http.StatusBadRequest, "INVALID_REQUEST")
	assertAdminError(t, missingErr, http.StatusNotFound, "NOT_FOUND")
	assertAdminError(t, malformedIDErr, http.StatusNotFound, "NOT_FOUND")
	assertAdminError(t, emptyErr, http.StatusBadRequest, "INVALID_REQUEST")
	var audits []models.OperationAudit
	require.NoError(t, TestSuite.DbConn.Order("created_at ASC").Find(&audits).Error)
	require.Len(t, audits, 3)
	assert.Equal(t, "admin.space.create", audits[0].Action)
	assert.Equal(t, "admin.space.update", audits[1].Action)
	assert.JSONEq(t, `{"changed":false,"fields":["name"]}`, string(audits[2].Metadata))
	for _, audit := range audits {
		assert.NotContains(t, string(audit.Metadata), "example.com")
		assert.NotContains(t, string(audit.Metadata), "map:capela")
	}
	var operationCount int64
	require.NoError(t, TestSuite.DbConn.Model(&models.AdminOperation{}).Count(&operationCount).Error)
	assert.Equal(t, int64(3), operationCount)
}

func TestAdminInstallationService_ActivityLifecycleValidationAndOriginalRetries(t *testing.T) {
	// given
	service := setupAdminInstallationTest(t)
	_, adminCtx := seedAdminInstallationUser(t, "admin-activity@example.com", userEntities.RoleAdmin, true)
	space, err := service.CreateSpace(adminCtx, uuid.NewString(), validCreateSpace("ginasio"))
	require.NoError(t, err)
	createKey := uuid.NewString()

	// when
	created, createErr := service.CreateActivity(adminCtx, createKey, validCreateActivity("desafio-foto", &space.ID))
	retried, retryErr := service.CreateActivity(adminCtx, createKey, validCreateActivity("desafio-foto", &space.ID))
	updated, updateErr := service.UpdateActivity(adminCtx, created.ID, uuid.NewString(), &messages.UpdateAdminActivityRequestDTO{SpaceID: nullOptional[string](), Description: nullOptional[string](), MomentPoints: optional(25), AllowsMoment: optional(false)})
	list, listErr := service.ListActivities(adminCtx, &messages.ListAdminActivitiesFilterDTO{})
	require.NoError(t, TestSuite.DbConn.Model(&models.Activity{}).Where("id = ?", created.ID).Update("status", "active").Error)
	_, activeArchiveErr := service.UpdateActivity(adminCtx, created.ID, uuid.NewString(), &messages.UpdateAdminActivityRequestDTO{Status: optional("archived")})
	require.NoError(t, TestSuite.DbConn.Model(&models.Activity{}).Where("id = ?", created.ID).Update("status", "paused").Error)
	archiveKey := uuid.NewString()
	archived, archiveErr := service.UpdateActivity(adminCtx, created.ID, archiveKey, &messages.UpdateAdminActivityRequestDTO{Status: optional("archived")})
	archiveRetry, archiveRetryErr := service.UpdateActivity(adminCtx, created.ID, archiveKey, &messages.UpdateAdminActivityRequestDTO{Status: optional("archived")})
	_, reconfigureArchivedErr := service.UpdateActivity(adminCtx, created.ID, uuid.NewString(), &messages.UpdateAdminActivityRequestDTO{Name: optional("Outro")})
	_, startSimulationErr := service.UpdateActivity(adminCtx, created.ID, uuid.NewString(), &messages.UpdateAdminActivityRequestDTO{Status: optional("active")})
	_, missingErr := service.UpdateActivity(adminCtx, uuid.NewString(), uuid.NewString(), &messages.UpdateAdminActivityRequestDTO{Name: optional("Ausente")})

	// then
	require.NoError(t, createErr)
	require.NoError(t, retryErr)
	require.NoError(t, updateErr)
	require.NoError(t, listErr)
	require.NoError(t, archiveErr)
	require.NoError(t, archiveRetryErr)
	assert.Equal(t, created, retried)
	assert.Equal(t, "draft", created.Status)
	assert.Nil(t, updated.SpaceID)
	assert.Nil(t, updated.Description)
	assert.Equal(t, 25, updated.MomentPoints)
	require.Len(t, list.Data, 1)
	assert.Equal(t, created.ID, list.Data[0].ID)
	assert.Equal(t, "archived", archived.Status)
	assert.Equal(t, archived, archiveRetry)
	assertAdminError(t, activeArchiveErr, http.StatusConflict, "ACTIVITY_STATE_CONFLICT")
	assertAdminError(t, reconfigureArchivedErr, http.StatusConflict, "ACTIVITY_STATE_CONFLICT")
	assertAdminError(t, startSimulationErr, http.StatusBadRequest, "INVALID_REQUEST")
	assertAdminError(t, missingErr, http.StatusNotFound, "NOT_FOUND")
	var stored models.Activity
	require.NoError(t, TestSuite.DbConn.First(&stored, "id = ?", created.ID).Error)
	assert.Equal(t, "archived", stored.Status)
}

func TestAdminInstallationService_RejectsInvalidSpaceAndActivityPayloads(t *testing.T) {
	// given
	service := setupAdminInstallationTest(t)
	_, adminCtx := seedAdminInstallationUser(t, "admin-validation@example.com", userEntities.RoleAdmin, true)
	valid := validCreateActivity("valid-activity", nil)
	badWindowStart := time.Now().UTC()
	badWindowEnd := badWindowStart.Add(-time.Minute)

	spaceCases := []struct {
		name    string
		request *messages.CreateAdminSpaceRequestDTO
	}{
		{"nil body", nil},
		{"missing slug", &messages.CreateAdminSpaceRequestDTO{Name: pointer("Nome")}},
		{"missing name", &messages.CreateAdminSpaceRequestDTO{Slug: pointer("valid")}},
		{"invalid slug", &messages.CreateAdminSpaceRequestDTO{Slug: pointer("Invalid Slug"), Name: pointer("Nome")}},
		{"blank name", &messages.CreateAdminSpaceRequestDTO{Slug: pointer("valid"), Name: pointer(" ")}},
		{"long map reference", &messages.CreateAdminSpaceRequestDTO{Slug: pointer("valid"), Name: pointer("Nome"), MapReference: optional(stringsOfLength(1001))}},
	}
	activityCases := []struct {
		name    string
		request *messages.CreateAdminActivityRequestDTO
		code    string
		status  int
	}{
		{"missing required", &messages.CreateAdminActivityRequestDTO{}, "INVALID_REQUEST", 400},
		{"invalid slug", cloneActivityRequest(valid, func(r *messages.CreateAdminActivityRequestDTO) { r.Slug = pointer("Invalid Slug") }), "INVALID_REQUEST", 400},
		{"blank name", cloneActivityRequest(valid, func(r *messages.CreateAdminActivityRequestDTO) { r.Name = pointer(" ") }), "INVALID_REQUEST", 400},
		{"long description", cloneActivityRequest(valid, func(r *messages.CreateAdminActivityRequestDTO) { r.Description = optional(stringsOfLength(4001)) }), "INVALID_REQUEST", 400},
		{"invalid kind", cloneActivityRequest(valid, func(r *messages.CreateAdminActivityRequestDTO) { r.Kind = pointer("unknown") }), "INVALID_REQUEST", 400},
		{"negative points", cloneActivityRequest(valid, func(r *messages.CreateAdminActivityRequestDTO) { r.CheckInPoints = pointer(-1) }), "INVALID_REQUEST", 400},
		{"invalid time window", cloneActivityRequest(valid, func(r *messages.CreateAdminActivityRequestDTO) {
			r.StartsAt = optional(badWindowStart)
			r.EndsAt = optional(badWindowEnd)
		}), "INVALID_REQUEST", 400},
		{"schedule moment", cloneActivityRequest(valid, func(r *messages.CreateAdminActivityRequestDTO) {
			r.Kind = pointer("schedule")
			r.AllowsMoment = pointer(true)
		}), "INVALID_REQUEST", 400},
		{"missing space", cloneActivityRequest(valid, func(r *messages.CreateAdminActivityRequestDTO) { r.SpaceID = optional(uuid.NewString()) }), "NOT_FOUND", 404},
		{"crossed space id", cloneActivityRequest(valid, func(r *messages.CreateAdminActivityRequestDTO) { r.SpaceID = optional("123") }), "NOT_FOUND", 404},
	}

	// when / then
	for _, testCase := range spaceCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := service.CreateSpace(adminCtx, uuid.NewString(), testCase.request)
			assertAdminError(t, err, http.StatusBadRequest, "INVALID_REQUEST")
		})
	}
	for _, testCase := range activityCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := service.CreateActivity(adminCtx, uuid.NewString(), testCase.request)
			assertAdminError(t, err, testCase.status, testCase.code)
		})
	}
}

func TestAdminInstallationService_DetailedPatchValidationAndEveryPersistedField(t *testing.T) {
	// given
	service := setupAdminInstallationTest(t)
	_, adminCtx := seedAdminInstallationUser(t, "admin-detailed-patch@example.com", userEntities.RoleAdmin, true)
	spaceA, err := service.CreateSpace(adminCtx, uuid.NewString(), validCreateSpace("space-a"))
	require.NoError(t, err)
	spaceB, err := service.CreateSpace(adminCtx, uuid.NewString(), validCreateSpace("space-b"))
	require.NoError(t, err)
	activityA, err := service.CreateActivity(adminCtx, uuid.NewString(), validCreateActivity("activity-a", &spaceA.ID))
	require.NoError(t, err)
	activityB, err := service.CreateActivity(adminCtx, uuid.NewString(), validCreateActivity("activity-b", nil))
	require.NoError(t, err)
	startsAt := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	endsAt := startsAt.Add(2 * time.Hour)

	// when
	updatedSpace, updateSpaceErr := service.UpdateSpace(adminCtx, spaceA.ID, uuid.NewString(), &messages.UpdateAdminSpaceRequestDTO{Slug: optional("space-a-renamed"), Name: optional("Space A Renamed"), MapReference: optional("map:new")})
	_, duplicateSpaceSlugErr := service.UpdateSpace(adminCtx, spaceA.ID, uuid.NewString(), &messages.UpdateAdminSpaceRequestDTO{Slug: optional("space-b")})
	_, nullSpaceSlugErr := service.UpdateSpace(adminCtx, spaceA.ID, uuid.NewString(), &messages.UpdateAdminSpaceRequestDTO{Slug: nullOptional[string]()})
	_, nullSpaceNameErr := service.UpdateSpace(adminCtx, spaceA.ID, uuid.NewString(), &messages.UpdateAdminSpaceRequestDTO{Name: nullOptional[string]()})
	_, longMapErr := service.UpdateSpace(adminCtx, spaceA.ID, uuid.NewString(), &messages.UpdateAdminSpaceRequestDTO{MapReference: optional(stringsOfLength(1001))})
	updatedActivity, updateActivityErr := service.UpdateActivity(adminCtx, activityA.ID, uuid.NewString(), &messages.UpdateAdminActivityRequestDTO{SpaceID: optional(spaceB.ID), Slug: optional("activity-a-renamed"), Name: optional("Activity A Renamed"), Description: optional("Nova descrição"), Kind: optional("competitive"), StartsAt: optional(startsAt), EndsAt: optional(endsAt), CheckInPoints: optional(30), MomentPoints: optional(40), CooldownSeconds: optional(90), AllowsMoment: optional(true)})
	_, duplicateActivitySlugErr := service.UpdateActivity(adminCtx, activityA.ID, uuid.NewString(), &messages.UpdateAdminActivityRequestDTO{Slug: optional(activityB.Slug)})
	_, malformedActivityIDErr := service.UpdateActivity(adminCtx, "123", uuid.NewString(), &messages.UpdateAdminActivityRequestDTO{Name: optional("X")})
	_, nilActivityRequestErr := service.UpdateActivity(adminCtx, activityA.ID, uuid.NewString(), nil)
	_, emptyActivityRequestErr := service.UpdateActivity(adminCtx, activityA.ID, uuid.NewString(), &messages.UpdateAdminActivityRequestDTO{})

	nullCases := []struct {
		name    string
		request *messages.UpdateAdminActivityRequestDTO
		status  int
		code    string
	}{
		{"kind null", &messages.UpdateAdminActivityRequestDTO{Kind: nullOptional[string]()}, 400, "INVALID_REQUEST"},
		{"check in null", &messages.UpdateAdminActivityRequestDTO{CheckInPoints: nullOptional[int]()}, 400, "INVALID_REQUEST"},
		{"moment null", &messages.UpdateAdminActivityRequestDTO{MomentPoints: nullOptional[int]()}, 400, "INVALID_REQUEST"},
		{"cooldown null", &messages.UpdateAdminActivityRequestDTO{CooldownSeconds: nullOptional[int]()}, 400, "INVALID_REQUEST"},
		{"allows moment null", &messages.UpdateAdminActivityRequestDTO{AllowsMoment: nullOptional[bool]()}, 400, "INVALID_REQUEST"},
		{"status null", &messages.UpdateAdminActivityRequestDTO{Status: nullOptional[string]()}, 400, "INVALID_REQUEST"},
		{"space missing", &messages.UpdateAdminActivityRequestDTO{SpaceID: optional(uuid.NewString())}, 404, "NOT_FOUND"},
		{"slug invalid", &messages.UpdateAdminActivityRequestDTO{Slug: optional("INVALID SLUG")}, 400, "INVALID_REQUEST"},
		{"name blank", &messages.UpdateAdminActivityRequestDTO{Name: optional(" ")}, 400, "INVALID_REQUEST"},
		{"description long", &messages.UpdateAdminActivityRequestDTO{Description: optional(stringsOfLength(4001))}, 400, "INVALID_REQUEST"},
		{"kind invalid", &messages.UpdateAdminActivityRequestDTO{Kind: optional("invalid")}, 400, "INVALID_REQUEST"},
		{"negative moment", &messages.UpdateAdminActivityRequestDTO{MomentPoints: optional(-1)}, 400, "INVALID_REQUEST"},
		{"schedule moment", &messages.UpdateAdminActivityRequestDTO{Kind: optional("schedule"), AllowsMoment: optional(true)}, 400, "INVALID_REQUEST"},
		{"window invalid", &messages.UpdateAdminActivityRequestDTO{StartsAt: optional(endsAt), EndsAt: optional(startsAt)}, 400, "INVALID_REQUEST"},
	}
	validationErrors := make([]error, len(nullCases))
	for index, testCase := range nullCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, validationErrors[index] = service.UpdateActivity(adminCtx, activityA.ID, uuid.NewString(), testCase.request)
		})
	}
	_, duplicateCreateErr := service.CreateActivity(adminCtx, uuid.NewString(), validCreateActivity(activityB.Slug, nil))
	_, nilCreateErr := service.CreateActivity(adminCtx, uuid.NewString(), nil)
	archivedNoOp, archivedNoOpErr := service.UpdateActivity(adminCtx, activityB.ID, uuid.NewString(), &messages.UpdateAdminActivityRequestDTO{Status: optional("archived")})
	archivedSecondNoOp, archivedSecondNoOpErr := service.UpdateActivity(adminCtx, activityB.ID, uuid.NewString(), &messages.UpdateAdminActivityRequestDTO{Status: optional("archived")})

	// then
	require.NoError(t, updateSpaceErr)
	require.NoError(t, updateActivityErr)
	require.NoError(t, archivedNoOpErr)
	require.NoError(t, archivedSecondNoOpErr)
	assert.Equal(t, "space-a-renamed", updatedSpace.Slug)
	assert.Equal(t, "map:new", *updatedSpace.MapReference)
	assert.Equal(t, spaceB.ID, *updatedActivity.SpaceID)
	assert.Equal(t, "activity-a-renamed", updatedActivity.Slug)
	assert.Equal(t, "competitive", updatedActivity.Kind)
	assert.Equal(t, 30, updatedActivity.CheckInPoints)
	assert.Equal(t, 40, updatedActivity.MomentPoints)
	assert.Equal(t, 90, updatedActivity.CooldownSeconds)
	assert.Equal(t, "archived", archivedNoOp.Status)
	assert.Equal(t, archivedNoOp, archivedSecondNoOp)
	assertAdminError(t, duplicateSpaceSlugErr, http.StatusConflict, "SLUG_ALREADY_EXISTS")
	assertAdminError(t, nullSpaceSlugErr, http.StatusBadRequest, "INVALID_REQUEST")
	assertAdminError(t, nullSpaceNameErr, http.StatusBadRequest, "INVALID_REQUEST")
	assertAdminError(t, longMapErr, http.StatusBadRequest, "INVALID_REQUEST")
	assertAdminError(t, duplicateActivitySlugErr, http.StatusConflict, "SLUG_ALREADY_EXISTS")
	assertAdminError(t, malformedActivityIDErr, http.StatusNotFound, "NOT_FOUND")
	assertAdminError(t, nilActivityRequestErr, http.StatusBadRequest, "INVALID_REQUEST")
	assertAdminError(t, emptyActivityRequestErr, http.StatusBadRequest, "INVALID_REQUEST")
	for index, validationErr := range validationErrors {
		assertAdminError(t, validationErr, nullCases[index].status, nullCases[index].code)
	}
	assertAdminError(t, duplicateCreateErr, http.StatusConflict, "SLUG_ALREADY_EXISTS")
	assertAdminError(t, nilCreateErr, http.StatusBadRequest, "INVALID_REQUEST")
}

func stringsOfLength(length int) string { return string(make([]byte, length)) }

func cloneActivityRequest(source *messages.CreateAdminActivityRequestDTO, mutate func(*messages.CreateAdminActivityRequestDTO)) *messages.CreateAdminActivityRequestDTO {
	clone := *source
	mutate(&clone)
	return &clone
}

func TestAdminInstallationService_RolesAssignmentsIsolationAndNoOps(t *testing.T) {
	// given
	service := setupAdminInstallationTest(t)
	_, adminCtx := seedAdminInstallationUser(t, "admin-assignment@example.com", userEntities.RoleAdmin, true)
	manager, _ := seedAdminInstallationUser(t, "manager-assignment@example.com", userEntities.RoleEventManager, true)
	incomplete, _ := seedAdminInstallationUser(t, "incomplete-assignment@example.com", userEntities.RoleEventManager, false)
	participant, _ := seedAdminInstallationUser(t, "participant-assignment@example.com", userEntities.RoleDefault, true)
	otherAdmin, _ := seedAdminInstallationUser(t, "other-admin-assignment@example.com", userEntities.RoleAdmin, true)
	activity, err := service.CreateActivity(adminCtx, uuid.NewString(), validCreateActivity("assignment-activity", nil))
	require.NoError(t, err)
	managerID := fmt.Sprint(manager.ID)

	// when
	assigned, assignErr := service.AssignManager(adminCtx, activity.ID, managerID, uuid.NewString())
	assignedNoOp, assignNoOpErr := service.AssignManager(adminCtx, activity.ID, managerID, uuid.NewString())
	managers, managersErr := service.ListManagers(adminCtx, activity.ID, &messages.ListAdminManagersFilterDTO{})
	_, demoteWithAssignmentErr := service.UpdateUserRole(adminCtx, managerID, uuid.NewString(), &messages.UpdateAdminUserRoleRequestDTO{Role: pointer("DEFAULT")})
	_, incompleteErr := service.AssignManager(adminCtx, activity.ID, fmt.Sprint(incomplete.ID), uuid.NewString())
	_, participantErr := service.AssignManager(adminCtx, activity.ID, fmt.Sprint(participant.ID), uuid.NewString())
	_, adminAssignmentErr := service.AssignManager(adminCtx, activity.ID, fmt.Sprint(otherAdmin.ID), uuid.NewString())
	adminRoleChange, adminRoleChangeErr := service.UpdateUserRole(adminCtx, fmt.Sprint(otherAdmin.ID), uuid.NewString(), &messages.UpdateAdminUserRoleRequestDTO{Role: pointer("DEFAULT")})
	removeKey := uuid.NewString()
	removeErr := service.RemoveManager(adminCtx, activity.ID, managerID, removeKey)
	removeRetryErr := service.RemoveManager(adminCtx, activity.ID, managerID, removeKey)
	removeNoOpErr := service.RemoveManager(adminCtx, activity.ID, managerID, uuid.NewString())
	demoted, demoteErr := service.UpdateUserRole(adminCtx, managerID, uuid.NewString(), &messages.UpdateAdminUserRoleRequestDTO{Role: pointer("DEFAULT")})
	promoted, promoteErr := service.UpdateUserRole(adminCtx, fmt.Sprint(participant.ID), uuid.NewString(), &messages.UpdateAdminUserRoleRequestDTO{Role: pointer("EVENT_MANAGER")})
	promoteNoOp, promoteNoOpErr := service.UpdateUserRole(adminCtx, fmt.Sprint(participant.ID), uuid.NewString(), &messages.UpdateAdminUserRoleRequestDTO{Role: pointer("EVENT_MANAGER")})
	_, invalidRoleErr := service.UpdateUserRole(adminCtx, fmt.Sprint(participant.ID), uuid.NewString(), &messages.UpdateAdminUserRoleRequestDTO{Role: pointer("ADMIN")})
	_, missingUserErr := service.AssignManager(adminCtx, activity.ID, "999999999", uuid.NewString())
	_, missingActivityErr := service.AssignManager(adminCtx, uuid.NewString(), fmt.Sprint(participant.ID), uuid.NewString())

	// then
	require.NoError(t, assignErr)
	require.NoError(t, assignNoOpErr)
	require.NoError(t, managersErr)
	require.NoError(t, removeErr)
	require.NoError(t, removeRetryErr)
	require.NoError(t, removeNoOpErr)
	require.NoError(t, demoteErr)
	require.NoError(t, promoteErr)
	require.NoError(t, promoteNoOpErr)
	assert.Equal(t, assigned, assignedNoOp)
	require.Len(t, managers.Data, 1)
	assert.Equal(t, managerID, managers.Data[0].ID.String())
	assert.Equal(t, "DEFAULT", demoted.Role)
	assert.Equal(t, "EVENT_MANAGER", promoted.Role)
	assert.Equal(t, promoted, promoteNoOp)
	assert.Nil(t, adminRoleChange)
	assertAdminError(t, demoteWithAssignmentErr, http.StatusConflict, "MANAGER_HAS_ASSIGNMENTS")
	assertAdminError(t, incompleteErr, http.StatusConflict, "MANAGER_NOT_ELIGIBLE")
	assertAdminError(t, participantErr, http.StatusConflict, "MANAGER_NOT_ELIGIBLE")
	assertAdminError(t, adminAssignmentErr, http.StatusConflict, "MANAGER_NOT_ELIGIBLE")
	assertAdminError(t, adminRoleChangeErr, http.StatusConflict, "ROLE_CHANGE_NOT_ALLOWED")
	assertAdminError(t, invalidRoleErr, http.StatusBadRequest, "INVALID_REQUEST")
	assertAdminError(t, missingUserErr, http.StatusNotFound, "NOT_FOUND")
	assertAdminError(t, missingActivityErr, http.StatusNotFound, "NOT_FOUND")
	var assignments int64
	require.NoError(t, TestSuite.DbConn.Model(&models.ActivityManagerAssignment{}).Count(&assignments).Error)
	assert.Zero(t, assignments)
}

func TestAdminInstallationService_ManagerAndRoleIdentifierErrors(t *testing.T) {
	// given
	service := setupAdminInstallationTest(t)
	_, adminCtx := seedAdminInstallationUser(t, "admin-identifiers@example.com", userEntities.RoleAdmin, true)
	manager, _ := seedAdminInstallationUser(t, "manager-identifiers@example.com", userEntities.RoleEventManager, true)
	activity, err := service.CreateActivity(adminCtx, uuid.NewString(), validCreateActivity("identifier-activity", nil))
	require.NoError(t, err)

	// when
	_, malformedListErr := service.ListManagers(adminCtx, "123", &messages.ListAdminManagersFilterDTO{})
	_, missingListErr := service.ListManagers(adminCtx, uuid.NewString(), &messages.ListAdminManagersFilterDTO{})
	_, malformedAssignActivityErr := service.AssignManager(adminCtx, "123", fmt.Sprint(manager.ID), uuid.NewString())
	_, malformedAssignUserErr := service.AssignManager(adminCtx, activity.ID, "0", uuid.NewString())
	malformedRemoveActivityErr := service.RemoveManager(adminCtx, "123", fmt.Sprint(manager.ID), uuid.NewString())
	malformedRemoveUserErr := service.RemoveManager(adminCtx, activity.ID, "not-user", uuid.NewString())
	missingRemoveUserErr := service.RemoveManager(adminCtx, activity.ID, "999999", uuid.NewString())
	missingRemoveActivityErr := service.RemoveManager(adminCtx, uuid.NewString(), fmt.Sprint(manager.ID), uuid.NewString())
	_, malformedRoleUserErr := service.UpdateUserRole(adminCtx, "0", uuid.NewString(), &messages.UpdateAdminUserRoleRequestDTO{Role: pointer("DEFAULT")})
	_, missingRoleUserErr := service.UpdateUserRole(adminCtx, "999999", uuid.NewString(), &messages.UpdateAdminUserRoleRequestDTO{Role: pointer("DEFAULT")})
	_, nilRoleRequestErr := service.UpdateUserRole(adminCtx, fmt.Sprint(manager.ID), uuid.NewString(), nil)
	_, nilRoleFieldErr := service.UpdateUserRole(adminCtx, fmt.Sprint(manager.ID), uuid.NewString(), &messages.UpdateAdminUserRoleRequestDTO{})
	_, nilStaffFilterErr := service.ListStaff(adminCtx, nil)
	_, deletedAdminCtx := seedAdminInstallationUser(t, "deleted-admin@example.com", userEntities.RoleAdmin, true)
	var deletedID uint64
	require.NoError(t, TestSuite.DbConn.Model(&models.User{}).Where("email = ?", "deleted-admin@example.com").Pluck("id", &deletedID).Error)
	require.NoError(t, TestSuite.DbConn.Delete(&models.User{}, deletedID).Error)
	_, deletedAdminErr := service.ListSpaces(deletedAdminCtx, &messages.ListAdminSpacesFilterDTO{})

	// then
	for _, testCase := range []struct {
		err    error
		status int
		code   string
	}{
		{malformedListErr, 404, "NOT_FOUND"}, {missingListErr, 404, "NOT_FOUND"},
		{malformedAssignActivityErr, 404, "NOT_FOUND"}, {malformedAssignUserErr, 404, "NOT_FOUND"},
		{malformedRemoveActivityErr, 404, "NOT_FOUND"}, {malformedRemoveUserErr, 404, "NOT_FOUND"},
		{missingRemoveUserErr, 404, "NOT_FOUND"}, {missingRemoveActivityErr, 404, "NOT_FOUND"},
		{malformedRoleUserErr, 404, "NOT_FOUND"}, {missingRoleUserErr, 404, "NOT_FOUND"},
		{nilRoleRequestErr, 400, "INVALID_REQUEST"}, {nilRoleFieldErr, 400, "INVALID_REQUEST"},
		{nilStaffFilterErr, 400, "INVALID_REQUEST"}, {deletedAdminErr, 401, "UNAUTHENTICATED"},
	} {
		assertAdminError(t, testCase.err, testCase.status, testCase.code)
	}
}

func TestAdminInstallationService_ConcurrentAssignmentRetriesHaveOneEffectAndAudit(t *testing.T) {
	// given
	service := setupAdminInstallationTest(t)
	_, adminCtx := seedAdminInstallationUser(t, "admin-concurrent@example.com", userEntities.RoleAdmin, true)
	manager, _ := seedAdminInstallationUser(t, "manager-concurrent@example.com", userEntities.RoleEventManager, true)
	activity, err := service.CreateActivity(adminCtx, uuid.NewString(), validCreateActivity("concurrent-activity", nil))
	require.NoError(t, err)
	key := uuid.NewString()
	const workers = 8
	errorsByWorker := make(chan error, workers)
	var waitGroup sync.WaitGroup
	waitGroup.Add(workers)

	// when
	for range workers {
		go func() {
			defer waitGroup.Done()
			_, err := service.AssignManager(adminCtx, activity.ID, fmt.Sprint(manager.ID), key)
			errorsByWorker <- err
		}()
	}
	waitGroup.Wait()
	close(errorsByWorker)

	// then
	for err := range errorsByWorker {
		require.NoError(t, err)
	}
	var assignments int64
	var audits int64
	require.NoError(t, TestSuite.DbConn.Model(&models.ActivityManagerAssignment{}).Count(&assignments).Error)
	require.NoError(t, TestSuite.DbConn.Model(&models.OperationAudit{}).Where("idempotency_key = ?", key).Count(&audits).Error)
	assert.Equal(t, int64(1), assignments)
	assert.Equal(t, int64(1), audits)
}

func TestAdminInstallationService_IdempotencyKeyCannotCrossManagerAndAdminOperations(t *testing.T) {
	// given
	service := setupAdminInstallationTest(t)
	_, adminCtx := seedAdminInstallationUser(t, "admin-cross-key@example.com", userEntities.RoleAdmin, true)
	activityID := seedActivity(t, activityEntities.StatusDraft)
	key := uuid.NewString()
	_, activityService := newIteration4Services()

	// when
	_, startErr := activityService.Start(adminCtx, activityID, key)
	_, adminErr := service.CreateSpace(adminCtx, key, validCreateSpace("cross-key"))

	// then
	require.NoError(t, startErr)
	assertAdminError(t, adminErr, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")
}
