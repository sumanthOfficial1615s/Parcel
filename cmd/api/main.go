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
	"parcel/internal/scanner"
	"parcel/internal/storage"
	"parcel/internal/tasks"
)

const (
	defaultServerAddr     = ":8080"
	defaultDBAddr         = "postgres://postgres:postgres@localhost:5433/files?sslmode=disable"
	defaultDBMaxOpenConns = 30
	defaultDBMaxIdleConns = 30
	defaultDBMaxIdleTime  = 15 * time.Minute
	defaultRedisAddr      = "localhost:6379"
	defaultClamAVAddr     = "localhost:3310"

	workerConcurrency = 5
)

// app holds the dependencies shared by the HTTP handlers.
type app struct {
	files   storage.FileStore
	objects storage.ObjectStore
	queue   tasks.Enqueuer
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found")
	}

	ctx := context.Background()

	minioConfig := storage.DefaultMinIOConfig()

	minioClient, err := storage.NewMinIOClient(ctx, minioConfig)
	if err != nil {
		log.Fatal("minio setup failed:", err)
	}

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

	redisOpt := asynq.RedisClientOpt{
		Addr: env.GetString("REDIS_ADDR", defaultRedisAddr),
	}

	queueClient := asynq.NewClient(redisOpt)
	defer queueClient.Close()

	if err := queueClient.Ping(); err != nil {
		log.Fatal("asynq redis connection failed:", err)
	}

	clamav := scanner.NewClamAV(
		env.GetString("CLAMAV_ADDR", defaultClamAVAddr),
	)

	if err := clamav.Ping(ctx); err != nil {
		log.Fatal("ClamAV connection failed:", err)
	}

	files := storage.NewPostgresFileStore(database)
	objects := storage.NewMinIOObjectStore(minioClient, minioConfig.Bucket)

	a := app{
		files:   files,
		objects: objects,
		queue:   queueClient,
	}

	worker := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: workerConcurrency,
		},
	)

	mux := asynq.NewServeMux()
	tasks.NewProcessor(files, objects, clamav).Register(mux)

	if err := worker.Start(mux); err != nil {
		log.Fatal("starting worker:", err)
	}
	defer worker.Shutdown()

	r := gin.Default()
	r.Use(cors.Default())

	r.POST("/upload", a.Upload)
	r.POST("/download", a.Download)

	addr := env.GetString("ADDR", defaultServerAddr)

	log.Println("listening on", addr)

	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
