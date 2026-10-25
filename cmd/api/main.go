package main

import (
	"context"
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
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
	storage      storage.Storage
	expiryClient *asynq.Client
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found")
	}

	ctx := context.Background()

	minioConfig := storage.DefaultLocalConfig()

	minioClient, err := storage.NewClient(ctx, minioConfig)
	if err != nil {
		log.Fatal("minio setup failed:", err)
	}

	database, err := db.New(
		env.GetString("DB_ADDR", defaultDBAddr),
		env.GetInt("DB_MAX_OPEN_CONN", defaultDBMaxOpenConns),
		env.GetInt("DB_MAX_IDLE_CONN", defaultDBMaxIdleConns),
		env.GetString(
			"DB_MAX_IDLE_TIME",
			defaultDBMaxIdleTime.String(),
		),
	)
	if err != nil {
		log.Fatal("database connection failed:", err)
	}
	defer database.Close()

	store := storage.NewStorage(
		database,
		minioClient,
		minioConfig.Bucket,
	)

	redisOpt := asynq.RedisClientOpt{
		Addr: env.GetString("REDIS_ADDR", "localhost:6378"),
	}

	expiryClient := asynq.NewClient(redisOpt)
	defer expiryClient.Close()

	if err := expiryClient.Ping(); err != nil {
		log.Fatal("asynq redis connection failed:", err)
	}

	a := app{
		storage:      store,
		expiryClient: expiryClient,
	}

	expiryServer := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: 5,
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(
		TypeDeleteExpiredFile,
		a.handleDeleteExpiredFile,
	)

	if err := expiryServer.Start(mux); err != nil {
		log.Fatal("starting expiry worker:", err)
	}
	defer expiryServer.Shutdown()

	r := gin.Default()
	r.Use(cors.Default())

	r.POST("/upload", a.Upload)
	r.POST("/download", a.Download)

	log.Println("listening on :8080")

	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
