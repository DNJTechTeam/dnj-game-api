package mappers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapTaskToEntity_NilInput(t *testing.T) {
	assert.Nil(t, MapTaskToEntity(nil))
}

func TestMapTaskEntityToModel_NilInput(t *testing.T) {
	assert.Nil(t, MapTaskEntityToModel(nil))
}

func TestMapUserToEntity_NilInput(t *testing.T) {
	assert.Nil(t, MapUserToEntity(nil))
}

func TestMapEntityToUser_NilInput(t *testing.T) {
	assert.Nil(t, MapEntityToUser(nil))
}
