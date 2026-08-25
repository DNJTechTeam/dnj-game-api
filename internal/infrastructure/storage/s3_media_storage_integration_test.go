package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMediaStorage_MinIOPrivateVersionedRoundTrip(t *testing.T) {
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "minio/minio:RELEASE.2025-07-23T15-54-02Z",
			ExposedPorts: []string{"9000/tcp"},
			Env: map[string]string{
				"MINIO_ROOT_USER":     "minioadmin",
				"MINIO_ROOT_PASSWORD": "minioadmin",
			},
			Cmd:        []string{"server", "/data"},
			WaitingFor: wait.ForHTTP("/minio/health/ready").WithPort(nat.Port("9000/tcp")),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, nat.Port("9000/tcp"))
	require.NoError(t, err)

	t.Setenv("S3_ENDPOINT", "http://"+host+":"+port.Port())
	t.Setenv("S3_REGION", "us-east-1")
	t.Setenv("S3_BUCKET", "dnj-media-test")
	t.Setenv("S3_ACCESS_KEY", "minioadmin")
	t.Setenv("S3_SECRET_KEY", "minioadmin")
	t.Setenv("S3_USE_PATH_STYLE", "true")
	t.Setenv("SERVER_ENVIRONMENT", "test")
	t.Setenv("DNJ_MEDIA_RETENTION_ANCHOR_AT", "2026-08-24T19:00:00Z")
	provider := NewS3MediaStorage().(*S3MediaStorage)
	require.NoError(t, provider.ValidateConfiguration())
	_, err = provider.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String("dnj-media-test")})
	require.NoError(t, err)
	_, err = provider.client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String("dnj-media-test"),
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusEnabled,
		},
	})
	require.NoError(t, err)

	body := []byte("private image bytes")
	digest := sha256.Sum256(body)
	checksum := base64.StdEncoding.EncodeToString(digest[:])
	now := time.Now().UTC()
	asset := &mediaEntities.Asset{
		ID:               "11111111-1111-4111-8111-111111111111",
		Bucket:           "dnj-media-test",
		StagingObjectKey: "staging/22222222-2222-4222-8222-222222222222",
		FinalObjectKey:   "media/33333333-3333-4333-8333-333333333333.jpg",
		ContentType:      "image/jpeg",
		Bytes:            int64(len(body)),
		ChecksumSHA256:   checksum,
		UploadExpiresAt:  now.Add(10 * time.Minute),
	}
	upload, err := provider.PresignUpload(ctx, asset, now)
	require.NoError(t, err)
	request, err := http.NewRequestWithContext(ctx, upload.Method, upload.URL, bytes.NewReader(body))
	require.NoError(t, err)
	for key, value := range upload.Headers {
		request.Header.Set(key, value)
	}
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)

	head, err := provider.HeadStaging(ctx, asset)
	require.NoError(t, err)
	assert.NotEmpty(t, head.VersionID)
	assert.Equal(t, checksum, head.ChecksumSHA256)
	downloaded, err := provider.DownloadStaging(ctx, asset, head.VersionID, 1024)
	require.NoError(t, err)
	assert.Equal(t, body, downloaded.Bytes)

	final, err := provider.PutFinal(ctx, asset, body, "image/jpeg")
	require.NoError(t, err)
	asset.FinalVersionID = &final.VersionID
	readURL, err := provider.PresignDownload(ctx, asset, now, 10*time.Minute)
	require.NoError(t, err)
	readResponse, err := http.Get(readURL)
	require.NoError(t, err)
	defer readResponse.Body.Close()
	readBody, err := io.ReadAll(readResponse.Body)
	require.NoError(t, err)
	assert.Equal(t, body, readBody)
	assert.Equal(t, "private, no-store", readResponse.Header.Get("Cache-Control"))

	require.NoError(t, provider.DeleteObjectVersions(ctx, asset.Bucket, asset.StagingObjectKey))
	require.NoError(t, provider.DeleteObjectVersions(ctx, asset.Bucket, asset.FinalObjectKey))
	_, err = provider.HeadFinal(ctx, asset, final.VersionID)
	assert.ErrorIs(t, err, mediaEntities.ErrObjectNotFound)
}
