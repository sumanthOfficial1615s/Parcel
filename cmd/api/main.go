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
	defaultDBAddr         = "postgres://postgres:postgres@localhost:5433/files?sslmode=disable"
	defaultDBMaxOpenConns = 30
	defaultDBMaxIdleConns = 30
	defaultDBMaxIdleTime  = 15 * time.Minute
)

type app struct {
	storage     storage.Storage
	fileHandler storage.FileHandler
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found")
	}

	ctx := context.Background()
	cfg := storage.DefaultLocalConfig()

	minioClient, err := storage.NewClient(ctx, cfg)
	if err != nil {
		log.Fatal("minio setup failed:", err)
	}

	fileHandler := storage.NewFileHandler(minioClient, cfg.Bucket)

	database, err := db.New(
		env.GetString("DB_ADDR", defaultDBAddr),
		env.GetInt("DB_MAX_OPEN_CONN", defaultDBMaxOpenConns),
		env.GetInt("DB_MAX_IDLE_CONN", defaultDBMaxIdleConns),
		env.GetString("DB_MAX_IDLE_TIME", defaultDBMaxIdleTime.String()),
	)
	if err != nil {
		log.Fatal("database connection failed:", err)
	}
	defer database.Close()

	store := storage.NewStorage(database)

	a := app{
		storage:     store,
		fileHandler: *fileHandler,
	}

	r := gin.Default()
	r.Use(cors.Default())
	r.POST("/upload", a.Upload)

	log.Println("listening on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
