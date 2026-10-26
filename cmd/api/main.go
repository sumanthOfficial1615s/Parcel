package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"

	"parcel/internal/auth"
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
	files     storage.FileStore
	users     storage.UserStore
	objects   storage.ObjectStore
	queue     tasks.Enqueuer
	jwtSecret string
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found")
	}

	ctx := context.Background()

	jwtSecret := env.GetString("JWT_SECRET", "")
	if err := auth.CheckSecret(jwtSecret); err != nil {
		log.Fatal(err)
	}

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
	users := storage.NewPostgresUserStore(database)
	objects := storage.NewMinIOObjectStore(minioClient, minioConfig.Bucket)

	a := app{
		files:     files,
		users:     users,
		objects:   objects,
		queue:     queueClient,
		jwtSecret: jwtSecret,
	}

	worker := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: workerConcurrency,
			// Background task failures never reach an HTTP response,
			// so log them here.
			ErrorHandler: asynq.ErrorHandlerFunc(
				func(ctx context.Context, task *asynq.Task, err error) {
					slog.Error("background task failed",
						"type", task.Type(),
						"error", err,
					)
				},
			),
		},
	)

	mux := asynq.NewServeMux()
	tasks.NewProcessor(files, objects, clamav).Register(mux)

	if err := worker.Start(mux); err != nil {
		log.Fatal("starting worker:", err)
	}
	defer worker.Shutdown()

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.LoggerWithFormatter(func(p gin.LogFormatterParams) string {
		path := p.Path
		if strings.HasPrefix(path, "/download/") {
			path = "/download/:token" // active download links must not end up in logs
		}
		return fmt.Sprintf("[GIN] %s | %d | %s | %s | %s %q\n",
			p.TimeStamp.Format("2006/01/02 - 15:04:05"),
			p.StatusCode, p.Latency, p.ClientIP, p.Method, path)
	}))

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AddAllowHeaders("Authorization")
	corsConfig.AddAllowMethods("DELETE")
	corsConfig.ExposeHeaders = []string{"Content-Disposition"}
	r.Use(cors.New(corsConfig))

	// public
	r.POST("/register", a.Register)
	r.POST("/login", a.Login)
	r.POST("/download", a.Download)           // checks the file password, returns a download URL
	r.GET("/download/:token", a.DownloadFile) // streams the file, supports Range (resume)

	// logged-in users only
	authed := r.Group("/", auth.Required(jwtSecret))
	authed.POST("/upload", a.Upload)
	authed.GET("/files", a.ListMyFiles)
	authed.DELETE("/files/:id", a.DeleteFile)

	addr := env.GetString("ADDR", defaultServerAddr)

	log.Println("listening on", addr)

	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
