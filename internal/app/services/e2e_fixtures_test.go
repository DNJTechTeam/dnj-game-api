package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// mustDecodeInto unmarshals an HTTP response body into v, failing the test
// with the raw body on error -- used instead of decodeJSONField whenever a
// journey needs more than a single string field back.
func mustDecodeInto(t *testing.T, raw []byte, v any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(raw, v), string(raw))
}

// seedActiveActivity walks the real admin HTTP surface (space create,
// activity create, manager assign, activity start) to produce a
// challenge-kind, moment-eligible activity that is already ACTIVE and
// assigned to managerID -- everything a manager journey needs before it can
// create a run. Returns the activity id.
func seedActiveActivity(
	t *testing.T,
	rec *e2eRecorder,
	adminToken string,
	slug string,
	managerID uint64,
	managerToken string,
) string {
	t.Helper()
	spaceBody := `{"slug":"` + slug + `-space","name":"Espaço ` + slug + `","mapReference":"map:` + slug + `"}`
	spaceResp := rec.call(
		"admin cria o espaço "+slug, "ADMIN", "admin", http.MethodPost, "/v2/admin/spaces",
		adminToken, uuid.NewString(), spaceBody,
		"POST /v2/admin/spaces cria um espaço com sucesso (201).",
	)
	require.Equal(t, http.StatusCreated, spaceResp.Code, spaceResp.Body.String())
	spaceID := decodeJSONField(t, spaceResp.Body.Bytes(), "id")

	// kind must be "competitive": that's the only kind the manager/games
	// surface (CreateRun/ListManageableGames) exposes today, and it's in the
	// allow-list for allows_moment=true, so it also supports the media/moment
	// reward flow the Default journey needs.
	activityBody := `{"spaceId":"` + spaceID + `","slug":"` + slug + `","name":"Atividade ` + slug + `","description":null,"kind":"competitive","startsAt":null,"endsAt":null,"checkInPoints":10,"momentPoints":20,"cooldownSeconds":60,"allowsMoment":true}`
	activityResp := rec.call(
		"admin cria a activity "+slug, "ADMIN", "admin", http.MethodPost, "/v2/admin/activities",
		adminToken, uuid.NewString(), activityBody,
		"POST /v2/admin/activities cria uma activity em draft (201).",
	)
	require.Equal(t, http.StatusCreated, activityResp.Code, activityResp.Body.String())
	activityID := decodeJSONField(t, activityResp.Body.Bytes(), "id")

	assignResp := rec.call(
		"admin atribui o gestor à activity "+slug, "ADMIN", "admin", http.MethodPut,
		"/v2/admin/activities/"+activityID+"/managers/"+uint64ToString(managerID), adminToken, uuid.NewString(), "",
		"PUT .../managers/:userId torna este Event Manager responsável pela activity (200).",
	)
	require.Equal(t, http.StatusOK, assignResp.Code, assignResp.Body.String())

	startResp := rec.call(
		"gestor inicia a activity "+slug, "EVENT_MANAGER", "manager", http.MethodPost,
		"/v2/manager/activities/"+activityID+"/start", managerToken, uuid.NewString(), "",
		"POST /v2/manager/activities/:id/start transiciona draft->active (200).",
	)
	require.Equal(t, http.StatusOK, startResp.Code, startResp.Body.String())

	return activityID
}

// promoteToEventManager uses the admin-only role endpoint to turn a DEFAULT
// user into an EVENT_MANAGER, exactly as an admin would from the staff
// screen.
func promoteToEventManager(t *testing.T, rec *e2eRecorder, adminToken string, userID uint64) {
	t.Helper()
	resp := rec.call(
		"admin promove usuário a EVENT_MANAGER", "ADMIN", "admin", http.MethodPatch,
		"/v2/admin/users/"+uint64ToString(userID)+"/role", adminToken, uuid.NewString(), `{"role":"EVENT_MANAGER"}`,
		"PATCH /v2/admin/users/:userId/role promove DEFAULT->EVENT_MANAGER (200).",
	)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
}

func uint64ToString(v uint64) string {
	return fmt.Sprint(v)
}
