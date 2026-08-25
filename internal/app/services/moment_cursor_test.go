package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMediaMoments_DecodeCursorRejectsMalformedAndTamperedCursors exercises every failure
// branch of the opaque, HMAC-signed pagination cursor directly: wrong shape, invalid signature
// encoding, a signature that does not match, an invalid payload encoding, malformed JSON, an
// unparseable capturedAt, and a non-UUID id. A cursor must never be trusted past any of these
// checks — feeding a tampered or malformed cursor back into ListMoments/ListModeration would
// otherwise let a client forge pagination state.
func TestMediaMoments_DecodeCursorRejectsMalformedAndTamperedCursors(t *testing.T) {
	mediaService, momentService, _ := setupMediaMomentServices(t)
	_ = mediaService

	empty, err := momentService.decodeCursor("")
	require.NoError(t, err)
	assert.Nil(t, empty)

	cases := map[string]string{
		"wrong shape (no dot)":            "not-a-cursor",
		"too many parts":                  "a.b.c",
		"invalid signature base64":        "cGF5bG9hZA.not-base64!!!",
		"signature does not match":        "cGF5bG9hZA.d3Jvbmctc2ln",
		"invalid payload base64":          signedCursor(t, momentService, "not valid base64 payload!!"),
		"malformed JSON payload":          signedCursor(t, momentService, base64.RawURLEncoding.EncodeToString([]byte("{not json"))),
		"capturedAt not RFC3339Nano":      signedCursor(t, momentService, base64.RawURLEncoding.EncodeToString([]byte(`{"capturedAt":"not-a-date","id":"11111111-1111-4111-8111-111111111111"}`))),
		"id is not a UUID":                signedCursor(t, momentService, base64.RawURLEncoding.EncodeToString([]byte(`{"capturedAt":"2026-01-01T00:00:00Z","id":"not-a-uuid"}`))),
	}
	for name, cursor := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := momentService.decodeCursor(cursor)
			assertAPIErrorCode(t, err, "INVALID_REQUEST")
		})
	}
}

func signedCursor(t *testing.T, service *MomentService, encodedPayload string) string {
	t.Helper()
	secret := service.cursorSecret()
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(encodedPayload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encodedPayload + "." + signature
}
