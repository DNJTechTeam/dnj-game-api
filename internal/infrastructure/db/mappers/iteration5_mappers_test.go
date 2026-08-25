package mappers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIteration5FavoriteMappers_NilSafety(t *testing.T) {
	// given / when / then
	assert.Nil(t, MapFavoriteEntityToModel(nil))
	assert.Nil(t, MapParticipantOperationToEntity(nil))
	assert.Nil(t, MapParticipantOperationEntityToModel(nil))
}
