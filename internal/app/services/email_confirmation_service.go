package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/scienceandcode/nucleus-api/internal/app/interfaces"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/common"
)

var (
	ErrTokenInvalid   = errors.New("Link de confirmação inválido ou expirado.")
	ErrTokenExpired   = errors.New("Link de confirmação expirado.")
	ErrTokenMalformed = errors.New("Link de confirmação inválido.")
)

type EmailConfirmationService struct {
	BaseService *BaseService
}

func NewEmailConfirmationService(baseService *BaseService) interfaces.EmailConfirmationServiceInterface {
	return &EmailConfirmationService{BaseService: baseService}
}

func (s *EmailConfirmationService) GenerateConfirmationToken(ctx context.Context, userID uint64, email string) (string, error) {
	issuedAt := time.Now()
	expiresAt := issuedAt.Add(96 * time.Hour)

	payload := fmt.Sprintf("%d|%s|%d|%d", userID, email, expiresAt.Unix(), issuedAt.Unix())

	payloadEncoded := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte(payload))

	secret := common.GetEnv("EMAIL_CONFIRMATION_SECRET")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payloadEncoded))
	signature := mac.Sum(nil)

	signatureEncoded := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(signature)

	token := fmt.Sprintf("%s|%s", payloadEncoded, signatureEncoded)

	return token, nil
}

func (s *EmailConfirmationService) ValidateConfirmationToken(ctx context.Context, tokenString string) (*interfaces.EmailConfirmationClaims, error) {
	parts := strings.Split(tokenString, "|")
	if len(parts) != 2 {
		return nil, ErrTokenMalformed
	}

	payloadEncoded := parts[0]
	signatureEncoded := parts[1]

	expectedSignature, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(signatureEncoded)
	if err != nil {
		return nil, ErrTokenMalformed
	}

	secret := common.GetEnv("EMAIL_CONFIRMATION_SECRET")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payloadEncoded))
	computedSignature := mac.Sum(nil)

	if !hmac.Equal(expectedSignature, computedSignature) {
		return nil, ErrTokenInvalid
	}

	payloadBytes, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(payloadEncoded)
	if err != nil {
		return nil, ErrTokenMalformed
	}

	payloadParts := strings.Split(string(payloadBytes), "|")
	if len(payloadParts) != 4 {
		return nil, ErrTokenMalformed
	}

	userID, err := strconv.ParseUint(payloadParts[0], 10, 64)
	if err != nil {
		return nil, ErrTokenMalformed
	}

	email := payloadParts[1]

	expiresAtUnix, err := strconv.ParseInt(payloadParts[2], 10, 64)
	if err != nil {
		return nil, ErrTokenMalformed
	}

	issuedAtUnix, err := strconv.ParseInt(payloadParts[3], 10, 64)
	if err != nil {
		return nil, ErrTokenMalformed
	}

	expiresAt := time.Unix(expiresAtUnix, 0)
	issuedAt := time.Unix(issuedAtUnix, 0)

	if time.Now().After(expiresAt) {
		return nil, ErrTokenExpired
	}

	claims := &interfaces.EmailConfirmationClaims{
		UserID:    strconv.FormatUint(userID, 10),
		Email:     email,
		ExpiresAt: expiresAt,
		IssuedAt:  issuedAt,
	}

	return claims, nil
}
