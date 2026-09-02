package email

import (
	"testing"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
	"github.com/stretchr/testify/require"
)

func TestEmailService_SendEmail_SkipsProviderOnLocalhost(t *testing.T) {
	// Given
	t.Setenv("SERVER_ENVIRONMENT", string(common.EnvironmentLocalhost))
	service := &EmailService{}

	// When
	err := service.SendEmail("user@example.com", "Subject", "Content")

	// Then
	require.NoError(t, err)
}
