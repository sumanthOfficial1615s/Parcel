package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"

	"parcel/internal/storage"
)

func main() {
	ctx := context.Background()
	cfg := storage.DefaultLocalConfig()

	client, err := storage.NewClient(ctx, cfg)
	if err != nil {
		log.Fatal("minio setup failed:", err)
	}
	fmt.Println("connected, bucket ready:", cfg.Bucket)

	content := "hello from the minio go client"
	reader := strings.NewReader(content)

	info, err := client.PutObject(ctx, cfg.Bucket, "test.txt", reader, int64(len(content)),
		minio.PutObjectOptions{ContentType: "text/plain"})
	if err != nil {
		log.Fatal("upload failed:", err)
	}
	fmt.Printf("uploaded %s (%d bytes)\n", info.Key, info.Size)

	presignedURL, err := client.PresignedGetObject(ctx, cfg.Bucket, "test.txt", 15*time.Minute, nil)
	if err != nil {
		log.Fatal("presign failed:", err)
	}
	fmt.Println("presigned download URL (valid 15 min):")
	fmt.Println(presignedURL.String())
}
