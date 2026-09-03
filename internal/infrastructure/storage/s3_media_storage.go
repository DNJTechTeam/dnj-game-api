package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	mediaInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/media/interfaces"
)

type S3MediaStorage struct {
	client                           *s3.Client
	config                           aws.Config
	endpoint, publicEndpoint, region string
	pathStyle                        bool
}

func NewS3MediaStorage() mediaInterfaces.Storage {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	region := strings.TrimSpace(os.Getenv("S3_REGION"))
	if region == "" {
		region = "us-east-1"
	}
	options := []func(*config.LoadOptions) error{config.WithRegion(region)}
	access, secret := os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY")
	if access != "" || secret != "" {
		options = append(
			options,
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(access, secret, "")),
		)
	}
	cfg, _ := config.LoadDefaultConfig(ctx, options...)
	endpoint := strings.TrimRight(strings.TrimSpace(os.Getenv("S3_ENDPOINT")), "/")
	publicEndpoint := strings.TrimRight(strings.TrimSpace(os.Getenv("S3_PUBLIC_ENDPOINT")), "/")
	if publicEndpoint == "" {
		publicEndpoint = endpoint
	}
	pathStyle, _ := strconv.ParseBool(os.Getenv("S3_USE_PATH_STYLE"))
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = pathStyle
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})
	return &S3MediaStorage{client: client, config: cfg, endpoint: endpoint, publicEndpoint: publicEndpoint, region: region, pathStyle: pathStyle}
}

func (s *S3MediaStorage) ValidateConfiguration() error {
	bucket := strings.TrimSpace(os.Getenv("S3_BUCKET"))
	retentionAnchor := strings.TrimSpace(os.Getenv("DNJ_MEDIA_RETENTION_ANCHOR_AT"))
	_, retentionErr := time.Parse(time.RFC3339, retentionAnchor)
	if bucket == "" || s.region == "" || retentionErr != nil || retentionAnchor == "" {
		return entities.ErrProviderUnavailable
	}
	for _, endpoint := range []string{s.endpoint, s.publicEndpoint} {
		if endpoint == "" {
			continue
		}
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Host == "" {
			return entities.ErrProviderUnavailable
		}
		if endpoint == s.publicEndpoint && os.Getenv("SERVER_ENVIRONMENT") != "localhost" && os.Getenv("SERVER_ENVIRONMENT") != "test" &&
			parsed.Scheme != "https" {
			return entities.ErrProviderUnavailable
		}
	}
	if s.config.Credentials == nil {
		return entities.ErrProviderUnavailable
	}
	access := strings.TrimSpace(os.Getenv("S3_ACCESS_KEY"))
	secret := strings.TrimSpace(os.Getenv("S3_SECRET_KEY"))
	if (access == "") != (secret == "") {
		return entities.ErrProviderUnavailable
	}
	return nil
}

func escapeKey(key string) string {
	parts := strings.Split(key, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}
func (s *S3MediaStorage) objectURL(bucket, key string) (string, error) {
	if s.publicEndpoint != "" {
		base, err := url.Parse(s.publicEndpoint)
		if err != nil {
			return "", err
		}
		if s.pathStyle {
			base.Path = strings.TrimRight(base.Path, "/") + "/" + url.PathEscape(bucket) + "/" + escapeKey(key)
		} else {
			base.Host = url.PathEscape(bucket) + "." + base.Host
			base.Path = strings.TrimRight(base.Path, "/") + "/" + escapeKey(key)
		}
		return base.String(), nil
	}
	host := "s3." + s.region + ".amazonaws.com"
	return "https://" + url.PathEscape(bucket) + "." + host + "/" + escapeKey(key), nil
}

func (s *S3MediaStorage) presign(
	ctx context.Context,
	method, bucket, key string,
	headers map[string]string,
	at time.Time,
	lifetime time.Duration,
) (string, error) {
	if err := s.ValidateConfiguration(); err != nil {
		return "", err
	}
	raw, err := s.objectURL(bucket, key)
	if err != nil {
		return "", entities.ErrProviderUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, method, raw, nil)
	if err != nil {
		return "", entities.ErrProviderUnavailable
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	query := req.URL.Query()
	query.Set("X-Amz-Expires", strconv.FormatInt(int64(lifetime/time.Second), 10))
	req.URL.RawQuery = query.Encode()
	creds, err := s.config.Credentials.Retrieve(ctx)
	if err != nil {
		return "", entities.ErrProviderUnavailable
	}
	signed, _, err := v4.NewSigner().
		PresignHTTP(ctx, creds, req, "UNSIGNED-PAYLOAD", "s3", s.region, at.UTC(), func(o *v4.SignerOptions) { o.DisableURIPathEscaping = true })
	if err != nil {
		return "", entities.ErrProviderUnavailable
	}
	return signed, nil
}

func (s *S3MediaStorage) PresignUpload(
	ctx context.Context,
	asset *entities.Asset,
	at time.Time,
) (*entities.PresignedRequest, error) {
	headers := map[string]string{"Content-Type": asset.ContentType, "X-Amz-Checksum-Sha256": asset.ChecksumSHA256}
	u, err := s.presign(ctx, http.MethodPut, asset.Bucket, asset.StagingObjectKey, headers, at, 10*time.Minute)
	if err != nil {
		return nil, err
	}
	return &entities.PresignedRequest{
		URL:       u,
		Method:    http.MethodPut,
		Headers:   headers,
		ExpiresAt: asset.UploadExpiresAt.UTC(),
	}, nil
}

func (s *S3MediaStorage) PresignDownload(
	ctx context.Context,
	asset *entities.Asset,
	at time.Time,
	lifetime time.Duration,
) (string, error) {
	if asset.FinalVersionID == nil || *asset.FinalVersionID == "" {
		return "", entities.ErrObjectNotFound
	}
	if lifetime > 5*time.Minute {
		lifetime = 5 * time.Minute
	}
	raw, err := s.objectURL(asset.Bucket, asset.FinalObjectKey)
	if err != nil {
		return "", entities.ErrProviderUnavailable
	}
	parsed, _ := url.Parse(raw)
	q := parsed.Query()
	q.Set("versionId", *asset.FinalVersionID)
	parsed.RawQuery = q.Encode()
	if err := s.ValidateConfiguration(); err != nil {
		return "", err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	query := req.URL.Query()
	query.Set("X-Amz-Expires", strconv.FormatInt(int64(lifetime/time.Second), 10))
	req.URL.RawQuery = query.Encode()
	creds, err := s.config.Credentials.Retrieve(ctx)
	if err != nil {
		return "", entities.ErrProviderUnavailable
	}
	signed, _, err := v4.NewSigner().
		PresignHTTP(ctx, creds, req, "UNSIGNED-PAYLOAD", "s3", s.region, at.UTC(), func(o *v4.SignerOptions) { o.DisableURIPathEscaping = true })
	if err != nil {
		return "", entities.ErrProviderUnavailable
	}
	return signed, nil
}

func storageError(err error) error {
	if err == nil {
		return nil
	}
	var api smithy.APIError
	if errors.As(err, &api) {
		code := strings.ToLower(api.ErrorCode())
		if code == "nosuchkey" || code == "notfound" || code == "nosuchversion" {
			return entities.ErrObjectNotFound
		}
	}
	return entities.ErrProviderUnavailable
}
func metadata(contentType *string, length *int64, checksum, version *string) *entities.ObjectMetadata {
	m := &entities.ObjectMetadata{}
	if contentType != nil {
		m.ContentType = *contentType
	}
	if length != nil {
		m.Bytes = *length
	}
	if checksum != nil {
		m.ChecksumSHA256 = *checksum
	}
	if version != nil {
		m.VersionID = *version
	}
	return m
}
func (s *S3MediaStorage) HeadStaging(ctx context.Context, asset *entities.Asset) (*entities.ObjectMetadata, error) {
	out, err := s.client.HeadObject(
		ctx,
		&s3.HeadObjectInput{
			Bucket:       aws.String(asset.Bucket),
			Key:          aws.String(asset.StagingObjectKey),
			ChecksumMode: types.ChecksumModeEnabled,
		},
	)
	if err != nil {
		return nil, storageError(err)
	}
	return metadata(out.ContentType, out.ContentLength, out.ChecksumSHA256, out.VersionId), nil
}

func (s *S3MediaStorage) DownloadStaging(
	ctx context.Context,
	asset *entities.Asset,
	version string,
	limit int64,
) (*entities.ObjectBody, error) {
	out, err := s.client.GetObject(
		ctx,
		&s3.GetObjectInput{
			Bucket:       aws.String(asset.Bucket),
			Key:          aws.String(asset.StagingObjectKey),
			VersionId:    aws.String(version),
			ChecksumMode: types.ChecksumModeEnabled,
		},
	)
	if err != nil {
		return nil, storageError(err)
	}
	defer out.Body.Close()
	body, err := io.ReadAll(io.LimitReader(out.Body, limit+1))
	if err != nil {
		return nil, entities.ErrProviderUnavailable
	}
	if int64(len(body)) > limit {
		return nil, entities.ErrObjectTooLarge
	}
	return &entities.ObjectBody{
		Metadata: *metadata(out.ContentType, out.ContentLength, out.ChecksumSHA256, out.VersionId),
		Bytes:    body,
	}, nil
}

func (s *S3MediaStorage) PutFinal(
	ctx context.Context,
	asset *entities.Asset,
	body []byte,
	contentType string,
) (*entities.ObjectMetadata, error) {
	digest := sha256.Sum256(body)
	checksum := base64.StdEncoding.EncodeToString(digest[:])
	out, err := s.client.PutObject(
		ctx,
		&s3.PutObjectInput{
			Bucket:         aws.String(asset.Bucket),
			Key:            aws.String(asset.FinalObjectKey),
			Body:           bytes.NewReader(body),
			ContentLength:  aws.Int64(int64(len(body))),
			ContentType:    aws.String(contentType),
			ChecksumSHA256: aws.String(checksum),
			CacheControl:   aws.String("private, no-store"),
		},
	)
	if err != nil {
		return nil, storageError(err)
	}
	if out.VersionId == nil || *out.VersionId == "" {
		return nil, entities.ErrProviderUnavailable
	}
	return &entities.ObjectMetadata{
		ContentType:    contentType,
		Bytes:          int64(len(body)),
		ChecksumSHA256: checksum,
		VersionID:      *out.VersionId,
	}, nil
}

func (s *S3MediaStorage) HeadFinal(
	ctx context.Context,
	asset *entities.Asset,
	version string,
) (*entities.ObjectMetadata, error) {
	out, err := s.client.HeadObject(
		ctx,
		&s3.HeadObjectInput{
			Bucket:       aws.String(asset.Bucket),
			Key:          aws.String(asset.FinalObjectKey),
			VersionId:    aws.String(version),
			ChecksumMode: types.ChecksumModeEnabled,
		},
	)
	if err != nil {
		return nil, storageError(err)
	}
	return metadata(out.ContentType, out.ContentLength, out.ChecksumSHA256, out.VersionId), nil
}
func (s *S3MediaStorage) DeleteObjectVersions(ctx context.Context, bucket, key string) error {
	var keyMarker, versionMarker *string
	for {
		out, err := s.client.ListObjectVersions(
			ctx,
			&s3.ListObjectVersionsInput{
				Bucket:          aws.String(bucket),
				Prefix:          aws.String(key),
				KeyMarker:       keyMarker,
				VersionIdMarker: versionMarker,
			},
		)
		if err != nil {
			return storageError(err)
		}
		objects := make([]types.ObjectIdentifier, 0, len(out.Versions)+len(out.DeleteMarkers))
		for _, item := range out.Versions {
			if aws.ToString(item.Key) == key {
				objects = append(objects, types.ObjectIdentifier{Key: item.Key, VersionId: item.VersionId})
			}
		}
		for _, item := range out.DeleteMarkers {
			if aws.ToString(item.Key) == key {
				objects = append(objects, types.ObjectIdentifier{Key: item.Key, VersionId: item.VersionId})
			}
		}
		if len(objects) > 0 {
			deleted, err := s.client.DeleteObjects(
				ctx,
				&s3.DeleteObjectsInput{
					Bucket: aws.String(bucket),
					Delete: &types.Delete{Objects: objects, Quiet: aws.Bool(true)},
				},
			)
			if err != nil {
				return storageError(err)
			}
			if len(deleted.Errors) > 0 {
				return entities.ErrProviderUnavailable
			}
		}
		if !aws.ToBool(out.IsTruncated) {
			break
		}
		keyMarker = out.NextKeyMarker
		versionMarker = out.NextVersionIdMarker
	}
	return nil
}

func RedactedStorageError(err error) string {
	if errors.Is(err, entities.ErrObjectNotFound) {
		return "not_found"
	}
	if errors.Is(err, entities.ErrObjectTooLarge) {
		return "too_large"
	}
	return "provider_unavailable"
}
