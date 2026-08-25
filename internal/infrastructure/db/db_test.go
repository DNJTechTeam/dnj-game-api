package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitAPI_DoesNotRequireReachableDatabase(t *testing.T) {
	// given
	SetConnection(nil)
	t.Setenv("DB_USER", "unreachable")
	t.Setenv("DB_PASSWORD", "unreachable")
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_PORT", "1")
	t.Setenv("DB_NAME", "unreachable")
	t.Setenv("SERVER_ENVIRONMENT", "test")
	t.Cleanup(func() { SetConnection(nil) })

	// when
	connection := InitAPI()

	// then
	require.NotNil(t, connection)
	sqlDB, err := connection.DB()
	require.NoError(t, err)
	assert.NoError(t, sqlDB.Close())
}
