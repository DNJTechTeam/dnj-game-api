package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/repositories"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/storage"
	mediaWorker "github.com/dnjtechteam/dnj-game-api/internal/worker/media"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	database := db.InitAPI()
	worker := mediaWorker.NewWorker(
		repositories.NewMediaRepository(database),
		storage.NewS3MediaStorage(),
	)

	if os.Getenv("DNJ_MEDIA_WORKER_MODE") != "loop" {
		if err := worker.RunOnce(context.Background()); err != nil {
			log.Fatalf("media cleanup cycle failed: %v", err)
		}
		return
	}
	intervalSeconds, err := strconv.Atoi(os.Getenv("DNJ_MEDIA_WORKER_INTERVAL_SECONDS"))
	if err != nil || intervalSeconds < 10 {
		intervalSeconds = 60
	}
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()
	for {
		if err := worker.RunOnce(context.Background()); err != nil {
			log.Printf("media cleanup cycle failed: %v", err)
		}
		<-ticker.C
	}
}
