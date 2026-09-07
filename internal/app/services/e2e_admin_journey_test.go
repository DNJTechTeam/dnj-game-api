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

// TestE2E_AdminJourney is the "superset" counterpart to
// TestE2E_EventManagerJourney: every action a manager can do, plus the
// admin-only installation/staff/moderation/broadcast surface, plus one
// concrete, executable proof that ADMIN always sees everything -- the same
// GET /v2/manager/runs/:id endpoint that returned 404 for a peer manager in
// the Event Manager journey returns 200 here for an Admin who was never
// assigned to that activity at all. Every call is recorded as evidence
// under docs/handoff/e2e-evidence/admin.json.
func TestE2E_AdminJourney(t *testing.T) {
	rig := setupE2ERig(t)
	rec := newE2ERecorder(t, "admin").withEngine(rig.Engine)

	admin, _ := seedIteration6User(t, "E2E Admin Journey Admin", userEntities.RoleAdmin, true, 0)
	managerX, _ := seedIteration6User(t, "E2E Admin Journey Manager X", userEntities.RoleDefault, true, 0)
	managerY, _ := seedIteration6User(t, "E2E Admin Journey Manager Y", userEntities.RoleDefault, true, 0)
	player, _ := seedIteration6User(t, "E2E Admin Journey Player", userEntities.RoleDefault, true, 0)

	adminToken, err := rig.JWT.GenerateIdentityToken(TestSuite.Ctx, admin)
	require.NoError(t, err)
	managerYToken, err := rig.JWT.GenerateIdentityToken(TestSuite.Ctx, managerY)
	require.NoError(t, err)
	playerToken, err := rig.JWT.GenerateIdentityToken(TestSuite.Ctx, player)
	require.NoError(t, err)

	healthResp := rec.call(
		"smoke check de healthcheck", "PUBLIC", "-", http.MethodGet, "/v2/healthcheck", "", "", "",
		"GET /v2/healthcheck responde sem autenticação (200).",
	)
	assert.Equal(t, http.StatusOK, healthResp.Code, healthResp.Body.String())

	promoteToEventManager(t, rec, adminToken, managerX.ID)
	promoteToEventManager(t, rec, adminToken, managerY.ID)

	// --- Staff catalog: Admin can list every combination, unlike a manager. ---
	allStaffResp := rec.call(
		"admin lista todo o staff (sem filtro)", "ADMIN", "admin", http.MethodGet,
		"/v2/admin/staff", adminToken, "", "",
		"GET /v2/admin/staff sem role lista ADMIN + EVENT_MANAGER juntos.",
	)
	require.Equal(t, http.StatusOK, allStaffResp.Code, allStaffResp.Body.String())
	var allStaff messages.PaginatedResponse[messages.AdminStaffResponseDTO]
	mustDecodeInto(t, allStaffResp.Body.Bytes(), &allStaff)
	roles := make(map[string]int)
	for _, member := range allStaff.Data {
		roles[member.Role]++
	}
	assert.Equal(t, 1, roles["ADMIN"])
	assert.Equal(t, 2, roles["EVENT_MANAGER"])

	adminOnlyResp := rec.call(
		"admin filtra staff por role=ADMIN", "ADMIN", "admin", http.MethodGet,
		"/v2/admin/staff?role=ADMIN", adminToken, "", "",
		"GET /v2/admin/staff?role=ADMIN só lista administradores.",
	)
	require.Equal(t, http.StatusOK, adminOnlyResp.Code, adminOnlyResp.Body.String())
	assert.Contains(t, adminOnlyResp.Body.String(), "E2E Admin Journey Admin")
	assert.NotContains(t, adminOnlyResp.Body.String(), "E2E Admin Journey Manager")

	// --- Activity/space CRUD + manager assignment for a SECOND manager. ---
	activityY := seedActiveActivity(t, rec, adminToken, "admin-journey-y", managerY.ID, managerYToken)

	runResp := rec.call(
		"gestor Y cria um run", "EVENT_MANAGER", "manager-y", http.MethodPost,
		"/v2/manager/runs", managerYToken, uuid.NewString(), `{"gameId":"`+activityY+`"}`,
		"POST /v2/manager/runs cria um run próprio do gestor Y (201).",
	)
	require.Equal(t, http.StatusCreated, runResp.Code, runResp.Body.String())
	var runY messages.ManagerRunResponseDTO
	mustDecodeInto(t, runResp.Body.Bytes(), &runY)

	// --- Superset proof: Admin was never assigned to activityY, yet sees runY. ---
	adminSeesRunResp := rec.call(
		"admin acessa o run do gestor Y sem estar atribuído", "ADMIN", "admin", http.MethodGet,
		"/v2/manager/runs/"+runY.ID, adminToken, "", "",
		"GET /v2/manager/runs/:id retorna 200 para Admin mesmo sem atribuição -- alçada global, o espelho do 404 que um gestor par recebe.",
	)
	assert.Equal(t, http.StatusOK, adminSeesRunResp.Code, adminSeesRunResp.Body.String())

	qrResp := rec.call(
		"gestor Y gera QR do run", "EVENT_MANAGER", "manager-y", http.MethodPost,
		"/v2/manager/runs/"+runY.ID+"/qr", managerYToken, uuid.NewString(), "",
		"POST .../qr emite um QR para check-in de jogadores (201).",
	)
	require.Equal(t, http.StatusCreated, qrResp.Code, qrResp.Body.String())
	var qr messages.QRResponseDTO
	mustDecodeInto(t, qrResp.Body.Bytes(), &qr)

	listManagersResp := rec.call(
		"admin lista gestores da activity Y", "ADMIN", "admin", http.MethodGet,
		"/v2/admin/activities/"+activityY+"/managers", adminToken, "", "",
		"GET .../managers lista o gestor Y como responsável.",
	)
	require.Equal(t, http.StatusOK, listManagersResp.Code, listManagersResp.Body.String())
	assert.Contains(t, listManagersResp.Body.String(), uint64ToString(managerY.ID))

	removeManagerResp := rec.call(
		"admin remove o gestor Y da activity", "ADMIN", "admin", http.MethodDelete,
		"/v2/admin/activities/"+activityY+"/managers/"+uint64ToString(managerY.ID), adminToken, uuid.NewString(), "",
		"DELETE .../managers/:userId desvincula o gestor (204).",
	)
	assert.Equal(t, http.StatusNoContent, removeManagerResp.Code, removeManagerResp.Body.String())

	demoteResp := rec.call(
		"admin rebaixa o gestor Y para DEFAULT", "ADMIN", "admin", http.MethodPatch,
		"/v2/admin/users/"+uint64ToString(managerY.ID)+"/role", adminToken, uuid.NewString(), `{"role":"DEFAULT"}`,
		"PATCH .../role rebaixa EVENT_MANAGER->DEFAULT depois que os assignments foram removidos (200).",
	)
	assert.Equal(t, http.StatusOK, demoteResp.Code, demoteResp.Body.String())

	// --- Player checks in, uploads two photos and creates two moments. ---
	joinResp := rec.call(
		"jogador faz check-in via QR", "DEFAULT", "player", http.MethodPost,
		"/v2/qr/validate", playerToken, uuid.NewString(), `{"qrToken":"`+qr.QRToken+`"}`,
		"POST /v2/qr/validate cria a Participation usada pelos moments seguintes.",
	)
	require.Equal(t, http.StatusCreated, joinResp.Code, joinResp.Body.String())
	var joined messages.ParticipationEnvelopeDTO
	mustDecodeInto(t, joinResp.Body.Bytes(), &joined)
	participationID := joined.Participation.ID

	// A Participation can back at most one Moment (moments_participation_unique),
	// so the second photo is a "free" moment (no participationId) -- a
	// spontaneous share rather than a challenge reward. That also exercises
	// both moderation queues: "challenge" for the first, "general" for the second.
	momentToApprove := createE2EMoment(t, rig, rec, "player", playerToken, participationID)
	momentToReject := createE2EMoment(t, rig, rec, "player", playerToken, "")

	challengeQueueResp := rec.call(
		"admin lista a fila de moderação (challenge)", "ADMIN", "admin", http.MethodGet,
		"/v2/admin/moments/moderation?queue=challenge&page=1", adminToken, "", "",
		"GET .../moderation?queue=challenge mostra o moment vinculado à participation, ainda pendente.",
	)
	require.Equal(t, http.StatusOK, challengeQueueResp.Code, challengeQueueResp.Body.String())
	assert.Contains(t, challengeQueueResp.Body.String(), momentToApprove)

	approveResp := rec.call(
		"admin aprova o primeiro moment", "ADMIN", "admin", http.MethodPost,
		"/v2/admin/moments/"+momentToApprove+"/moderation", adminToken, uuid.NewString(), `{"action":"approve"}`,
		"POST .../moderation com action=approve tira o moment da fila pendente (200).",
	)
	require.Equal(t, http.StatusOK, approveResp.Code, approveResp.Body.String())
	assert.Contains(t, approveResp.Body.String(), `"moderationStatus":"approved"`)

	generalQueueResp := rec.call(
		"admin lista a fila de moderação (general)", "ADMIN", "admin", http.MethodGet,
		"/v2/admin/moments/moderation?queue=general&page=1", adminToken, "", "",
		"GET .../moderation?queue=general mostra o moment espontâneo, ainda pendente.",
	)
	require.Equal(t, http.StatusOK, generalQueueResp.Code, generalQueueResp.Body.String())
	assert.Contains(t, generalQueueResp.Body.String(), momentToReject)

	rejectResp := rec.call(
		"admin rejeita o segundo moment", "ADMIN", "admin", http.MethodPost,
		"/v2/admin/moments/"+momentToReject+"/moderation", adminToken, uuid.NewString(), `{"action":"delete_photo"}`,
		"POST .../moderation com action=delete_photo derruba a foto e rejeita o moment (200).",
	)
	require.Equal(t, http.StatusOK, rejectResp.Code, rejectResp.Body.String())
	assert.Contains(t, rejectResp.Body.String(), `"photoStatus":"deleted"`)

	// --- Group invite lifecycle (create -> renew -> revoke). ---
	groupResp := rec.call(
		"jogador cria o próprio grupo", "DEFAULT", "player", http.MethodPost,
		"/v2/groups", playerToken, uuid.NewString(), `{"name":"Grupo E2E Admin"}`,
		"POST /v2/groups é self-service -- qualquer autenticado pode criar (201).",
	)
	require.Equal(t, http.StatusCreated, groupResp.Code, groupResp.Body.String())
	groupID := decodeJSONField(t, groupResp.Body.Bytes(), "id")

	inviteResp := rec.call(
		"admin cria um convite para o grupo", "ADMIN", "admin", http.MethodPost,
		"/v2/admin/groups/"+groupID+"/invites", adminToken, uuid.NewString(), "",
		"POST .../invites cria um convite de uso único (201).",
	)
	require.Equal(t, http.StatusCreated, inviteResp.Code, inviteResp.Body.String())
	inviteID := decodeJSONField(t, inviteResp.Body.Bytes(), "id")

	renewResp := rec.call(
		"admin renova o convite", "ADMIN", "admin", http.MethodPost,
		"/v2/admin/groups/"+groupID+"/invites/"+inviteID+"/renew", adminToken, uuid.NewString(), "",
		"POST .../renew revoga o convite antigo e emite um novo (201).",
	)
	require.Equal(t, http.StatusCreated, renewResp.Code, renewResp.Body.String())
	renewedInviteID := decodeJSONField(t, renewResp.Body.Bytes(), "id")

	revokeResp := rec.call(
		"admin revoga o convite renovado", "ADMIN", "admin", http.MethodDelete,
		"/v2/admin/groups/"+groupID+"/invites/"+renewedInviteID, adminToken, "", "",
		"DELETE .../invites/:inviteId revoga o convite (204).",
	)
	assert.Equal(t, http.StatusNoContent, revokeResp.Code, revokeResp.Body.String())

	listInvitesResp := rec.call(
		"admin lista convites do grupo", "ADMIN", "admin", http.MethodGet,
		"/v2/admin/groups/"+groupID+"/invites", adminToken, "", "",
		"GET .../invites lista o histórico (original + renovado, ambos revogados).",
	)
	require.Equal(t, http.StatusOK, listInvitesResp.Code, listInvitesResp.Body.String())

	// --- Broadcast notification reaches the player. ---
	broadcastResp := rec.call(
		"admin dispara notificação broadcast", "ADMIN", "admin", http.MethodPost,
		"/v2/admin/notifications", adminToken, uuid.NewString(), `{"title":"Aviso geral","body":"Bem-vindos à jornada E2E."}`,
		"POST /v2/admin/notifications sem targetUserIds alcança todo mundo com announcement habilitado (201).",
	)
	require.Equal(t, http.StatusCreated, broadcastResp.Code, broadcastResp.Body.String())

	playerNotificationsResp := rec.call(
		"jogador vê a notificação recebida", "DEFAULT", "player", http.MethodGet,
		"/v2/notifications", playerToken, "", "",
		"GET /v2/notifications do jogador contém o broadcast enviado pelo admin.",
	)
	require.Equal(t, http.StatusOK, playerNotificationsResp.Code, playerNotificationsResp.Body.String())
	assert.Contains(t, playerNotificationsResp.Body.String(), "Aviso geral")
}

// createE2EMoment uploads a tiny valid JPEG through the real media HTTP
// surface, completes it, and creates a public moment. When participationID
// is non-empty the moment is tied to it (origin=challenge); otherwise it's a
// free/spontaneous moment (origin=free) -- a Participation can only ever
// back one Moment, so a second photo in the same journey must go this route.
// Returns the moment id.
func createE2EMoment(t *testing.T, rig *e2eRig, rec *e2eRecorder, actorLabel, token, participationID string) string {
	t.Helper()
	body := mediaMomentImage(t, "image/jpeg")

	intentResp := rec.call(
		actorLabel+" pede uma intenção de upload", "DEFAULT", actorLabel, http.MethodPost,
		"/v2/media/upload-intents", token, uuid.NewString(),
		`{"contentType":"image/jpeg","bytes":`+uint64ToString(uint64(len(body)))+`,"checksumSha256":"`+checksum(body)+`"}`,
		"POST /v2/media/upload-intents devolve uma URL assinada para upload direto ao storage (201).",
	)
	require.Equal(t, http.StatusCreated, intentResp.Code, intentResp.Body.String())
	intentID := decodeJSONField(t, intentResp.Body.Bytes(), "id")
	rig.Storage.staged[intentID] = body

	completeResp := rec.call(
		actorLabel+" confirma o upload", "DEFAULT", actorLabel, http.MethodPost,
		"/v2/media/"+intentID+"/complete", token, uuid.NewString(), "",
		"POST /v2/media/:id/complete valida a imagem e marca o asset como available (200).",
	)
	require.Equal(t, http.StatusOK, completeResp.Code, completeResp.Body.String())
	assetID := decodeJSONField(t, completeResp.Body.Bytes(), "id")

	momentBody := `{"mediaAssetId":"` + assetID + `","publishConsent":true}`
	if participationID != "" {
		momentBody = `{"mediaAssetId":"` + assetID + `","publishConsent":true,"participationId":"` + participationID + `"}`
	}
	momentResp := rec.call(
		actorLabel+" publica um moment", "DEFAULT", actorLabel, http.MethodPost,
		"/v2/moments", token, uuid.NewString(), momentBody,
		"POST /v2/moments nasce pending e já aparece no feed (201).",
	)
	require.Equal(t, http.StatusCreated, momentResp.Code, momentResp.Body.String())
	return decodeJSONField(t, momentResp.Body.Bytes(), "id")
}
