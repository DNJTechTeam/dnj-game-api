package storage

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestS3MediaStorage_PresignUploadSignsReturnedHeaders(t *testing.T) {
	// Given an S3 upload signed with temporary credentials, as in Lambda.
	t.Setenv("S3_BUCKET", "dnj-media-test")
	t.Setenv("S3_ACCESS_KEY", "")
	t.Setenv("S3_SECRET_KEY", "")
	t.Setenv("DNJ_MEDIA_RETENTION_ANCHOR_AT", "2026-08-24T19:00:00Z")
	provider := &S3MediaStorage{
		region: "sa-east-1",
		config: aws.Config{
			Credentials: credentials.NewStaticCredentialsProvider("test-key", "test-secret", "test-session"),
		},
	}
	now := time.Date(2026, 9, 3, 14, 0, 0, 0, time.UTC)
	digest := sha256.Sum256([]byte("image bytes"))
	asset := &entities.Asset{
		Bucket:           "dnj-media-test",
		StagingObjectKey: "staging/test-upload",
		ContentType:      "image/jpeg",
		ChecksumSHA256:   base64.StdEncoding.EncodeToString(digest[:]),
		UploadExpiresAt:  now.Add(10 * time.Minute),
	}

	// When the API generates the URL and headers sent by the frontend.
	upload, err := provider.PresignUpload(context.Background(), asset, now)
	require.NoError(t, err)
	parsed, err := url.Parse(upload.URL)
	require.NoError(t, err)
	query := parsed.Query()

	// Then every returned header is signed, with no checksum hoisted to the URL.
	assert.Equal(t, http.MethodPut, upload.Method)
	assert.Equal(t, asset.ChecksumSHA256, upload.Headers["X-Amz-Checksum-Sha256"])
	signedHeaders := strings.Split(query.Get("X-Amz-SignedHeaders"), ";")
	for header := range upload.Headers {
		assert.Contains(t, signedHeaders, strings.ToLower(header))
	}
	for key := range query {
		assert.False(t, strings.EqualFold(key, "x-amz-checksum-sha256"))
	}
	assert.Equal(t, "test-session", query.Get("X-Amz-Security-Token"))
	assert.Equal(t, "600", query.Get("X-Amz-Expires"))
}
