package main

import (
	"context"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db"
	push "github.com/dnjtechteam/dnj-game-api/internal/worker/push"
	"github.com/joho/godotenv"
	"log"
)

func main() {
	_ = godotenv.Load()
	if err := push.NewWorker(db.InitAPI()).RunOnce(context.Background()); err != nil {
		log.Fatal(err)
	}
}
