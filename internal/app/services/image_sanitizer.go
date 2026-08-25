package services

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
)

const maxImagePixels int64 = 20_000_000
const maxImageDimension = 16_384
const maxMediaBytes int64 = 10 * 1024 * 1024

var errInvalidImage = errors.New("invalid image")

func sanitizeImage(body []byte, contentType string) ([]byte, error) {
	if int64(len(body)) > maxMediaBytes || len(body) < 8 {
		return nil, errInvalidImage
	}
	expected := "jpeg"
	if contentType == "image/png" {
		expected = "png"
	}
	if expected == "jpeg" {
		if len(body) < 3 || body[0] != 0xff || body[1] != 0xd8 || body[2] != 0xff {
			return nil, errInvalidImage
		}
	} else if !bytes.Equal(body[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) {
		return nil, errInvalidImage
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || format != expected || config.Width <= 0 || config.Height <= 0 ||
		config.Width > maxImageDimension ||
		config.Height > maxImageDimension ||
		int64(config.Width)*int64(config.Height) > maxImagePixels {
		return nil, errInvalidImage
	}
	reader := bytes.NewReader(body)
	var decoded image.Image
	if expected == "jpeg" {
		decoded, err = jpeg.Decode(reader)
	} else {
		decoded, err = png.Decode(reader)
	}
	if err != nil || decoded.Bounds().Dx() != config.Width || decoded.Bounds().Dy() != config.Height {
		return nil, errInvalidImage
	}
	var output bytes.Buffer
	limited := &limitWriter{writer: &output, remaining: maxMediaBytes + 1}
	if expected == "jpeg" {
		err = jpeg.Encode(limited, decoded, &jpeg.Options{Quality: 90})
	} else {
		encoder := png.Encoder{CompressionLevel: png.DefaultCompression}
		err = encoder.Encode(limited, decoded)
	}
	if err != nil || int64(output.Len()) > maxMediaBytes {
		return nil, errInvalidImage
	}
	return output.Bytes(), nil
}

type limitWriter struct {
	writer    io.Writer
	remaining int64
}

func (w *limitWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, errInvalidImage
	}
	n, err := w.writer.Write(p)
	w.remaining -= int64(n)
	return n, err
}
