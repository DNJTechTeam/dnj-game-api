package services

import (
	"net/http"
	"testing"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_DefaultJourney walks the real HTTP surface as a brand-new player:
// self-service email signup, onboarding, joining a group via an
// admin-issued invite, browsing content, favoriting an activity, checking
// into a run via QR, sharing a moment (which publishes immediately, per the
// "curate everything, block nothing" moderation model), liking, checking
// rankings/notifications, and finally proving the admin-only surface
// rejects it. Every call is recorded as evidence under
// docs/handoff/e2e-evidence/default.json.
func TestE2E_DefaultJourney(t *testing.T) {
	rig := setupE2ERig(t)
	rec := newE2ERecorder(t, "default").withEngine(rig.Engine)
	rig.Identity.signupCode = fixedCode("482910")

	admin, _ := seedIteration6User(t, "E2E Default Journey Admin", userEntities.RoleAdmin, true, 0)
	manager, _ := seedIteration6User(t, "E2E Default Journey Manager", userEntities.RoleDefault, true, 0)
	adminToken, err := rig.JWT.GenerateIdentityToken(TestSuite.Ctx, admin)
	require.NoError(t, err)
	managerToken, err := rig.JWT.GenerateIdentityToken(TestSuite.Ctx, manager)
	require.NoError(t, err)
	promoteToEventManager(t, rec, adminToken, manager.ID)

	const playerEmail = "e2e-default-player@example.com"

	signupResp := rec.call(
		"jogador pede o signup por e-mail", "PUBLIC", "player", http.MethodPost,
		"/v2/auth/signup", "", "", `{"email":"`+playerEmail+`"}`,
		"POST /v2/auth/signup dispara um código de verificação por e-mail (200).",
	)
	require.Equal(t, http.StatusOK, signupResp.Code, signupResp.Body.String())

	verifyResp := rec.call(
		"jogador confirma o código", "PUBLIC", "player", http.MethodPost,
		"/v2/auth/signup/verify", "", "", `{"email":"`+playerEmail+`","code":"482910"}`,
		"POST /v2/auth/signup/verify troca o código por uma sessão (accessToken) e cria a conta (200).",
	)
	require.Equal(t, http.StatusOK, verifyResp.Code, verifyResp.Body.String())
	var session messages.IdentitySessionResponseDTO
	mustDecodeInto(t, verifyResp.Body.Bytes(), &session)
	require.NotEmpty(t, session.AccessToken)
	playerToken := session.AccessToken
	assert.True(t, session.OnboardingRequired)

	currentResp := rec.call(
		"jogador confere a própria sessão", "DEFAULT", "player", http.MethodGet,
		"/v2/auth/session", playerToken, "", "",
		"GET /v2/auth/session confirma a identidade (200).",
	)
	require.Equal(t, http.StatusOK, currentResp.Code, currentResp.Body.String())

	onboardResp := rec.call(
		"jogador completa o onboarding (sem grupo)", "DEFAULT", "player", http.MethodPatch,
		"/v2/auth/onboarding", playerToken, "", `{"document":"52998224725","mobilePhone":"5541999990000"}`,
		"PATCH /v2/auth/onboarding aceita groupId omitido -- escolher um grupo é opcional e pode vir depois (200).",
	)
	require.Equal(t, http.StatusOK, onboardResp.Code, onboardResp.Body.String())
	var onboarded messages.CurrentSessionResponseDTO
	mustDecodeInto(t, onboardResp.Body.Bytes(), &onboarded)
	assert.False(t, onboarded.OnboardingRequired)

	// --- Join a group after the fact, via an admin-issued invite. ---
	group := seedGroup(t, "E2E Default Journey Group")
	inviteResp := rec.call(
		"admin cria um convite para o grupo", "ADMIN", "admin", http.MethodPost,
		"/v2/admin/groups/"+uint64ToString(group.ID)+"/invites", adminToken, uuid.NewString(), "",
		"POST .../invites cria um convite de uso único (201).",
	)
	require.Equal(t, http.StatusCreated, inviteResp.Code, inviteResp.Body.String())
	inviteCode := decodeJSONField(t, inviteResp.Body.Bytes(), "code")

	consumeResp := rec.call(
		"jogador consome o convite e entra no grupo", "DEFAULT", "player", http.MethodPost,
		"/v2/groups/invites/consume", playerToken, uuid.NewString(), `{"code":"`+inviteCode+`"}`,
		"POST /v2/groups/invites/consume vincula o jogador ao grupo do convite (200).",
	)
	require.Equal(t, http.StatusOK, consumeResp.Code, consumeResp.Body.String())

	groupMeResp := rec.call(
		"jogador vê o próprio grupo", "DEFAULT", "player", http.MethodGet,
		"/v2/groups/me", playerToken, "", "",
		"GET /v2/groups/me confirma a associação (200).",
	)
	require.Equal(t, http.StatusOK, groupMeResp.Code, groupMeResp.Body.String())
	assert.Contains(t, groupMeResp.Body.String(), "E2E Default Journey Group")

	membersResp := rec.call(
		"jogador lista os membros do grupo", "DEFAULT", "player", http.MethodGet,
		"/v2/groups/me/members", playerToken, "", "",
		"GET /v2/groups/me/members lista o próprio jogador como membro (200).",
	)
	require.Equal(t, http.StatusOK, membersResp.Code, membersResp.Body.String())

	profileUpdateResp := rec.call(
		"jogador atualiza o próprio perfil", "DEFAULT", "player", http.MethodPatch,
		"/v2/users/me", playerToken, uuid.NewString(), `{"name":"Jogador E2E"}`,
		"PATCH /v2/users/me só permite editar name e mobilePhone (200).",
	)
	require.Equal(t, http.StatusOK, profileUpdateResp.Code, profileUpdateResp.Body.String())
	assert.Contains(t, profileUpdateResp.Body.String(), "Jogador E2E")

	// --- Browse content and favorite an activity. ---
	activityID := seedActiveActivity(t, rec, adminToken, "default-journey", manager.ID, managerToken)

	scheduleResp := rec.call(
		"jogador vê a agenda pública", "DEFAULT", "player", http.MethodGet,
		"/v2/schedule", playerToken, "", "",
		"GET /v2/schedule lista activities publicadas, sem exigir autenticação (200).",
	)
	require.Equal(t, http.StatusOK, scheduleResp.Code, scheduleResp.Body.String())

	activitiesResp := rec.call(
		"jogador lista activities", "DEFAULT", "player", http.MethodGet,
		"/v2/activities", playerToken, "", "",
		"GET /v2/activities lista o catálogo público (200).",
	)
	require.Equal(t, http.StatusOK, activitiesResp.Code, activitiesResp.Body.String())

	favoriteResp := rec.call(
		"jogador favorita a activity", "DEFAULT", "player", http.MethodPut,
		"/v2/users/me/favorites/"+activityID, playerToken, uuid.NewString(), "",
		"PUT .../favorites/:activityId marca a activity como favorita (204).",
	)
	require.Equal(t, http.StatusNoContent, favoriteResp.Code, favoriteResp.Body.String())

	listFavoritesResp := rec.call(
		"jogador lista os favoritos", "DEFAULT", "player", http.MethodGet,
		"/v2/users/me/favorites?page=1", playerToken, "", "",
		"GET .../favorites confirma a activity favoritada (200).",
	)
	require.Equal(t, http.StatusOK, listFavoritesResp.Code, listFavoritesResp.Body.String())
	assert.Contains(t, listFavoritesResp.Body.String(), activityID)

	// --- Check in via QR (manager creates the run/QR, player scans it). ---
	runResp := rec.call(
		"gestor cria o run para o jogador entrar", "EVENT_MANAGER", "manager", http.MethodPost,
		"/v2/manager/runs", managerToken, uuid.NewString(), `{"gameId":"`+activityID+`"}`,
		"POST /v2/manager/runs cria o run que o jogador vai escanear (201).",
	)
	require.Equal(t, http.StatusCreated, runResp.Code, runResp.Body.String())
	var run messages.ManagerRunResponseDTO
	mustDecodeInto(t, runResp.Body.Bytes(), &run)

	qrResp := rec.call(
		"gestor gera o QR do run", "EVENT_MANAGER", "manager", http.MethodPost,
		"/v2/manager/runs/"+run.ID+"/qr", managerToken, uuid.NewString(), "",
		"POST .../qr emite o QR que o jogador vai validar (201).",
	)
	require.Equal(t, http.StatusCreated, qrResp.Code, qrResp.Body.String())
	var qr messages.QRResponseDTO
	mustDecodeInto(t, qrResp.Body.Bytes(), &qr)

	joinResp := rec.call(
		"jogador faz check-in via QR", "DEFAULT", "player", http.MethodPost,
		"/v2/qr/validate", playerToken, uuid.NewString(), `{"qrToken":"`+qr.QRToken+`"}`,
		"POST /v2/qr/validate cria a Participation do jogador no run (201).",
	)
	require.Equal(t, http.StatusCreated, joinResp.Code, joinResp.Body.String())
	var joined messages.ParticipationEnvelopeDTO
	mustDecodeInto(t, joinResp.Body.Bytes(), &joined)

	currentRunResp := rec.call(
		"jogador vê o próprio run atual", "DEFAULT", "player", http.MethodGet,
		"/v2/activity-runs/current", playerToken, "", "",
		"GET /v2/activity-runs/current mostra o run em que o jogador está participando (200).",
	)
	require.Equal(t, http.StatusOK, currentRunResp.Code, currentRunResp.Body.String())

	currentParticipationResp := rec.call(
		"jogador vê a própria participação atual", "DEFAULT", "player", http.MethodGet,
		"/v2/participations/current", playerToken, "", "",
		"GET /v2/participations/current mostra a Participation recém-criada (200).",
	)
	require.Equal(t, http.StatusOK, currentParticipationResp.Code, currentParticipationResp.Body.String())

	// --- Share a moment: it publishes immediately (pending, but already in the feed). ---
	momentID := createE2EMoment(t, rig, rec, "player", playerToken, joined.Participation.ID)

	feedResp := rec.call(
		"jogador vê o próprio moment no feed", "DEFAULT", "player", http.MethodGet,
		"/v2/moments?scope=feed", playerToken, "", "",
		"GET /v2/moments?scope=feed já mostra o moment (pending e approved aparecem -- só rejected some).",
	)
	require.Equal(t, http.StatusOK, feedResp.Code, feedResp.Body.String())
	assert.Contains(t, feedResp.Body.String(), momentID)

	likeResp := rec.call(
		"jogador curte o próprio moment", "DEFAULT", "player", http.MethodPost,
		"/v2/moments/"+momentID+"/likes", playerToken, uuid.NewString(), "",
		"POST /v2/moments/:momentId/likes alterna a curtida (200).",
	)
	require.Equal(t, http.StatusOK, likeResp.Code, likeResp.Body.String())
	assert.Contains(t, likeResp.Body.String(), `"liked":true`)

	// --- Rankings and notifications. ---
	overviewResp := rec.call(
		"jogador vê seu overview no jogo", "DEFAULT", "player", http.MethodGet,
		"/v2/game/overview", playerToken, "", "",
		"GET /v2/game/overview mostra rankings e o histórico de pontos do jogador (200).",
	)
	require.Equal(t, http.StatusOK, overviewResp.Code, overviewResp.Body.String())

	rankingsResp := rec.call(
		"jogador vê o ranking individual", "DEFAULT", "player", http.MethodGet,
		"/v2/rankings?scope=individual&page=1", playerToken, "", "",
		"GET /v2/rankings é público mas também acessível autenticado (200).",
	)
	require.Equal(t, http.StatusOK, rankingsResp.Code, rankingsResp.Body.String())

	prefsResp := rec.call(
		"jogador vê as preferências de notificação", "DEFAULT", "player", http.MethodGet,
		"/v2/notifications/preferences", playerToken, "", "",
		"GET /v2/notifications/preferences retorna os defaults (200).",
	)
	require.Equal(t, http.StatusOK, prefsResp.Code, prefsResp.Body.String())

	updatePrefsResp := rec.call(
		"jogador desliga notificações de pontos", "DEFAULT", "player", http.MethodPut,
		"/v2/notifications/preferences", playerToken, uuid.NewString(), `{"pointsEnabled":false}`,
		"PUT /v2/notifications/preferences aceita atualização parcial (200).",
	)
	require.Equal(t, http.StatusOK, updatePrefsResp.Code, updatePrefsResp.Body.String())

	broadcastResp := rec.call(
		"admin avisa a todos (para o jogador ler)", "ADMIN", "admin", http.MethodPost,
		"/v2/admin/notifications", adminToken, uuid.NewString(), `{"title":"Bem-vindo","body":"Sua jornada começou."}`,
		"POST /v2/admin/notifications alcança o jogador recém-cadastrado (201).",
	)
	require.Equal(t, http.StatusCreated, broadcastResp.Code, broadcastResp.Body.String())

	listNotificationsResp := rec.call(
		"jogador lista as próprias notificações", "DEFAULT", "player", http.MethodGet,
		"/v2/notifications", playerToken, "", "",
		"GET /v2/notifications mostra o aviso do admin, ainda não lido (200).",
	)
	require.Equal(t, http.StatusOK, listNotificationsResp.Code, listNotificationsResp.Body.String())
	var notifications messages.NotificationListResponseDTO
	mustDecodeInto(t, listNotificationsResp.Body.Bytes(), &notifications)
	require.NotEmpty(t, notifications.Data)
	notificationID := notifications.Data[0].ID

	markReadResp := rec.call(
		"jogador marca a notificação como lida", "DEFAULT", "player", http.MethodPost,
		"/v2/notifications/"+notificationID+"/read", playerToken, uuid.NewString(), "",
		"POST .../read marca a notificação como lida (200).",
	)
	require.Equal(t, http.StatusOK, markReadResp.Code, markReadResp.Body.String())

	// --- Negative proof: the admin-only surface rejects a DEFAULT player. ---
	forbiddenResp := rec.call(
		"jogador tenta acessar a tela de staff", "DEFAULT", "player", http.MethodGet,
		"/v2/admin/staff", playerToken, "", "",
		"GET /v2/admin/staff rejeita um jogador comum com 403 FORBIDDEN.",
	)
	assert.Equal(t, http.StatusForbidden, forbiddenResp.Code, forbiddenResp.Body.String())
}
