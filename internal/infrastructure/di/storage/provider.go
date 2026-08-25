package storage

import (
	mediaInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/media/interfaces"
	storageImplementation "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/storage"
)

func ProvideMediaStorage() mediaInterfaces.Storage {
	return storageImplementation.NewS3MediaStorage()
}
