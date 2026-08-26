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

// TestE2E_EventManagerJourney walks the real HTTP surface as an Event
// Manager: get assigned an activity, run its full lifecycle (create -> QR
// -> start/pause/resume -> finalize), and prove the two halves of the
// jurisdiction contract that matter for this role: (1) it only ever sees
// activities/runs it is assigned to, never a peer manager's, and (2) every
// admin-only surface (staff, spaces, moderation, broadcast) rejects it with
// 403 FORBIDDEN. Every call is recorded as evidence under
// docs/handoff/e2e-evidence/event_manager.json.
func TestE2E_EventManagerJourney(t *testing.T) {
	rig := setupE2ERig(t)
	rec := newE2ERecorder(t, "event_manager").withEngine(rig.Engine)

	admin, _ := seedMediaMomentUser(t, "e2e-em-admin@example.com", userEntities.RoleAdmin, true)
	managerA, _ := seedMediaMomentUser(t, "e2e-em-manager-a@example.com", userEntities.RoleDefault, true)
	managerB, _ := seedMediaMomentUser(t, "e2e-em-manager-b@example.com", userEntities.RoleDefault, true)

	adminToken, err := rig.JWT.GenerateIdentityToken(TestSuite.Ctx, admin)
	require.NoError(t, err)
	managerAToken, err := rig.JWT.GenerateIdentityToken(TestSuite.Ctx, managerA)
	require.NoError(t, err)
	managerBToken, err := rig.JWT.GenerateIdentityToken(TestSuite.Ctx, managerB)
	require.NoError(t, err)

	promoteToEventManager(t, rec, adminToken, managerA.ID)
	promoteToEventManager(t, rec, adminToken, managerB.ID)

	activityA := seedActiveActivity(t, rec, adminToken, "em-journey-a", managerA.ID, managerAToken)
	activityB := seedActiveActivity(t, rec, adminToken, "em-journey-b", managerB.ID, managerBToken)

	// Manager A's own-scope overview must show activity A and must not leak activity B.
	overviewResp := rec.call(
		"gestor A vê seu próprio painel", "EVENT_MANAGER", "manager-a", http.MethodGet,
		"/v2/manager/game-overview", managerAToken, "", "",
		"GET /v2/manager/game-overview só lista activities atribuídas a este gestor (não vê a activity do gestor B).",
	)
	require.Equal(t, http.StatusOK, overviewResp.Code, overviewResp.Body.String())
	var overview messages.ManagerGameOverviewResponseDTO
	mustDecodeInto(t, overviewResp.Body.Bytes(), &overview)
	gameIDs := make([]string, len(overview.Actions.Games))
	for i, game := range overview.Actions.Games {
		gameIDs[i] = game.ID
	}
	assert.Contains(t, gameIDs, activityA)
	assert.NotContains(t, gameIDs, activityB)

	// Manager A runs its own activity end to end.
	createResp := rec.call(
		"gestor A cria um run", "EVENT_MANAGER", "manager-a", http.MethodPost,
		"/v2/manager/runs", managerAToken, uuid.NewString(), `{"gameId":"`+activityA+`"}`,
		"POST /v2/manager/runs cria um run para uma activity própria (201).",
	)
	require.Equal(t, http.StatusCreated, createResp.Code, createResp.Body.String())
	var runA messages.ManagerRunResponseDTO
	mustDecodeInto(t, createResp.Body.Bytes(), &runA)

	qrResp := rec.call(
		"gestor A gera QR do run", "EVENT_MANAGER", "manager-a", http.MethodPost,
		"/v2/manager/runs/"+runA.ID+"/qr", managerAToken, uuid.NewString(), "",
		"POST .../qr emite um QR válido para check-in (201).",
	)
	require.Equal(t, http.StatusCreated, qrResp.Code, qrResp.Body.String())

	ownRunResp := rec.call(
		"gestor A vê o detalhe do próprio run", "EVENT_MANAGER", "manager-a", http.MethodGet,
		"/v2/manager/runs/"+runA.ID, managerAToken, "", "",
		"GET /v2/manager/runs/:id retorna 200 para o run do próprio gestor.",
	)
	require.Equal(t, http.StatusOK, ownRunResp.Code, ownRunResp.Body.String())

	startResp := rec.call(
		"gestor A inicia o run", "EVENT_MANAGER", "manager-a", http.MethodPost,
		"/v2/manager/runs/"+runA.ID+"/start", managerAToken, uuid.NewString(), "",
		"POST .../start transiciona scheduled->active (200).",
	)
	require.Equal(t, http.StatusOK, startResp.Code, startResp.Body.String())

	pauseResp := rec.call(
		"gestor A pausa o run", "EVENT_MANAGER", "manager-a", http.MethodPost,
		"/v2/manager/runs/"+runA.ID+"/pause", managerAToken, uuid.NewString(), "",
		"POST .../pause transiciona active->paused (200).",
	)
	require.Equal(t, http.StatusOK, pauseResp.Code, pauseResp.Body.String())

	resumeResp := rec.call(
		"gestor A retoma o run", "EVENT_MANAGER", "manager-a", http.MethodPost,
		"/v2/manager/runs/"+runA.ID+"/resume", managerAToken, uuid.NewString(), "",
		"POST .../resume transiciona paused->active (200).",
	)
	require.Equal(t, http.StatusOK, resumeResp.Code, resumeResp.Body.String())

	finalizeResp := rec.call(
		"gestor A finaliza o run (sem participantes)", "EVENT_MANAGER", "manager-a", http.MethodPost,
		"/v2/manager/runs/"+runA.ID+"/results", managerAToken, uuid.NewString(), `{"results":[]}`,
		"POST .../results finaliza o run mesmo sem check-ins (200).",
	)
	require.Equal(t, http.StatusOK, finalizeResp.Code, finalizeResp.Body.String())

	// Manager B runs its own activity, just enough to have a run to withhold from A.
	createBResp := rec.call(
		"gestor B cria um run na própria activity", "EVENT_MANAGER", "manager-b", http.MethodPost,
		"/v2/manager/runs", managerBToken, uuid.NewString(), `{"gameId":"`+activityB+`"}`,
		"POST /v2/manager/runs cria um run para a activity do gestor B (201).",
	)
	require.Equal(t, http.StatusCreated, createBResp.Code, createBResp.Body.String())
	var runB messages.ManagerRunResponseDTO
	mustDecodeInto(t, createBResp.Body.Bytes(), &runB)

	// --- Jurisdiction proof: a manager never sees a peer's run. ---
	crossRunResp := rec.call(
		"gestor A tenta ver o run do gestor B", "EVENT_MANAGER", "manager-a", http.MethodGet,
		"/v2/manager/runs/"+runB.ID, managerAToken, "", "",
		"GET /v2/manager/runs/:id de um run alheio retorna 404 -- o escopo do gestor é só o que ele gerencia.",
	)
	assert.Equal(t, http.StatusNotFound, crossRunResp.Code, crossRunResp.Body.String())

	// --- Negative proof: every admin-only surface rejects a manager. ---
	for _, negative := range []struct {
		label, method, path, body string
	}{
		{"listar staff", http.MethodGet, "/v2/admin/staff", ""},
		{"criar espaço", http.MethodPost, "/v2/admin/spaces", `{"slug":"x","name":"x","mapReference":"x"}`},
		{"moderar um moment", http.MethodPost, "/v2/admin/moments/" + uuid.NewString() + "/moderation", `{"action":"approve"}`},
		{"disparar notificação broadcast", http.MethodPost, "/v2/admin/notifications", `{"title":"x","body":"x"}`},
	} {
		resp := rec.call(
			"gestor tenta "+negative.label, "EVENT_MANAGER", "manager-a", negative.method, negative.path,
			managerAToken, uuid.NewString(), negative.body,
			"Superfície admin-only rejeita Event Manager com 403 FORBIDDEN ("+negative.label+").",
		)
		assert.Equal(t, http.StatusForbidden, resp.Code, negative.label+": "+resp.Body.String())
	}
}
