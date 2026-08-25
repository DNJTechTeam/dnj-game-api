package services

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMediaMoments_SanitizerAcceptsSupportedImagesAndRemovesMetadata(t *testing.T) {
	jpegBody := mediaMomentImage(t, "image/jpeg")
	exifPayload := []byte("Exif\x00\x00GPSLatitude=private")
	segment := make([]byte, 4+len(exifPayload))
	segment[0], segment[1] = 0xff, 0xe1
	binary.BigEndian.PutUint16(segment[2:4], uint16(len(exifPayload)+2))
	copy(segment[4:], exifPayload)
	withExif := append(append(append([]byte{}, jpegBody[:2]...), segment...), jpegBody[2:]...)

	sanitizedJPEG, err := sanitizeImage(withExif, "image/jpeg")
	require.NoError(t, err)
	assert.False(t, bytes.Contains(sanitizedJPEG, []byte("Exif")))
	assert.False(t, bytes.Contains(sanitizedJPEG, []byte("GPSLatitude")))

	pngBody := mediaMomentImage(t, "image/png")
	sanitizedPNG, err := sanitizeImage(pngBody, "image/png")
	require.NoError(t, err)
	assert.Equal(t, []byte{0x89, 'P', 'N', 'G'}, sanitizedPNG[:4])
}

func TestMediaMoments_SanitizerRejectsDisguisedMalformedAndOversizedImages(t *testing.T) {
	jpegBody := mediaMomentImage(t, "image/jpeg")
	pngBody := mediaMomentImage(t, "image/png")

	for name, test := range map[string]struct {
		body        []byte
		contentType string
	}{
		"false mime":      {jpegBody, "image/png"},
		"bad magic":       {[]byte("not an image"), "image/jpeg"},
		"truncated jpeg":  {jpegBody[:len(jpegBody)/2], "image/jpeg"},
		"truncated png":   {pngBody[:len(pngBody)/2], "image/png"},
		"too many pixels": {pngHeader(10_000, 2_001), "image/png"},
		"dimension limit": {pngHeader(maxImageDimension+1, 1), "image/png"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := sanitizeImage(test.body, test.contentType)
			assert.ErrorIs(t, err, errInvalidImage)
		})
	}
}

func pngHeader(width, height int) []byte {
	result := append([]byte{}, []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}...)
	payload := make([]byte, 13)
	binary.BigEndian.PutUint32(payload[0:4], uint32(width))
	binary.BigEndian.PutUint32(payload[4:8], uint32(height))
	payload[8] = 8
	payload[9] = 2
	chunk := append([]byte("IHDR"), payload...)
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(payload)))
	crc := make([]byte, 4)
	binary.BigEndian.PutUint32(crc, crc32.ChecksumIEEE(chunk))
	result = append(result, length...)
	result = append(result, chunk...)
	result = append(result, crc...)
	return result
}

func TestMediaMoments_RetentionAndChecksumRules(t *testing.T) {
	t.Setenv("DNJ_MEDIA_RETENTION_ANCHOR_AT", "2026-08-20T23:00:00-03:00")
	due, err := retentionDueAt(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 11, 19, 2, 0, 0, 0, time.UTC), due)

	createdAfterAnchor := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	due, err = retentionDueAt(createdAfterAnchor)
	require.NoError(t, err)
	assert.Equal(t, createdAfterAnchor.Add(90*24*time.Hour), due)

	t.Setenv("DNJ_MEDIA_RETENTION_ANCHOR_AT", "local-time-is-invalid")
	_, err = retentionDueAt(mediaMomentNow)
	assertAPIErrorCode(t, err, "MEDIA_UNAVAILABLE")

	valid := checksum([]byte("same"))
	assert.True(t, secureChecksumEqual(valid, valid))
	assert.False(t, secureChecksumEqual(valid, base64.StdEncoding.EncodeToString([]byte("different"))))
}
