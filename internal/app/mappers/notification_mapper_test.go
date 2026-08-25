package mappers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotificationMapper_NilInputs(t *testing.T) {
	assert.Nil(t, MapNotificationToResponseDTO(nil))
	assert.Nil(t, MapNotificationPreferencesToResponseDTO(nil))
	assert.Empty(t, MapNotificationsToResponseDTOs(nil))
}
