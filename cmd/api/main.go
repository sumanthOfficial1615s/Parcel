package main

import (
	"context"
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"parcel/internal/db"
	"parcel/internal/env"
	"parcel/internal/storage"
)

const (
	DB_MAX_OPEN_CONN = 30
	DB_MAX_IDLE_CONN = 30
	DB_MAX_IDLE_TIME = 15 * time.Minute
)

type app struct {
	storage     storage.Storage
	fileHandler FileHandler
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found")
	}

	ctx := context.Background()

	cfg := storage.DefaultLocalConfig()

	client, err := storage.NewClient(ctx, cfg)
	if err != nil {
		log.Fatal("MinIO setup failed:", err)
	}

	handler := NewFileHandler(client, cfg.Bucket)

	db, err := db.New(
		env.GetString("DB_ADDR", "5433"),
		env.GetInt("DB_MAX_OPEN_CONN", 30),
		env.GetInt("DB_MAX_IDLE_CONN", 30),
		env.GetString("DB_MAX_IDLE_TIME", "15m"),
	)

	if err != nil {
		log.Fatal("database connection failed:", err)
	}

	defer db.Close()

	store := storage.NewStorage(db)

	app := app{
		storage:     store,
		fileHandler: *handler,
	}

	r := gin.Default()

	r.Use(cors.Default())

	r.POST("/upload", app.Upload)

	log.Println("listening on :8080")

	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
