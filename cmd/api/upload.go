package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
)

type FileHandler struct {
	Client *minio.Client
	Bucket string
}

func NewFileHandler(client *minio.Client, bucket string) *FileHandler {
	return &FileHandler{Client: client, Bucket: bucket}
}

func (a *app) Upload(c *gin.Context) {
	reader, err := c.Request.MultipartReader()
	if err != nil {
		c.String(http.StatusBadRequest, "expected multipart form")
		return
	}

	var password, expiresAt string
	fileHandled := false
	ctx := c.Request.Context()

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			c.String(http.StatusBadRequest, "malformed multipart")
			return
		}

		switch part.FormName() {

		case "password":
			buf, err := io.ReadAll(part)
			if err != nil {
				c.String(http.StatusBadRequest, "could not read password field")
				return
			}
			password = string(buf)

		case "expires_at":
			buf, err := io.ReadAll(part)
			if err != nil {
				c.String(http.StatusBadRequest, "could not read expires_at field")
				return
			}
			expiresAt = string(buf)

		default:
			if password == "" {
				c.String(http.StatusBadRequest, "password must be sent before the file part")
				return
			}
			if expiresAt == "" {
				c.String(http.StatusBadRequest, "expires_at must be sent before the file part")
				return
			}
			if part.FileName() == "" {
				continue
			}

			fileHandled = true

			pr, pw := io.Pipe()
			hasher := sha256.New()

			go func() {
				defer pw.Close()
				mw := io.MultiWriter(pw, hasher)
				io.Copy(mw, part)
			}()

			info, err := a.fileHandler.Client.PutObject(ctx, a.fileHandler.Bucket, part.FileName(), pr, -1, minio.PutObjectOptions{})

			if err != nil {
				c.String(http.StatusInternalServerError, "upload failed")
				return
			}

			hash := hex.EncodeToString(hasher.Sum(nil))
			c.JSON(http.StatusOK, gin.H{
				"filename":   info.Key,
				"size":       info.Size,
				"sha256":     hash,
				"password":   password,
				"expires_at": expiresAt,
			})
		}
	}

	if !fileHandled {
		c.String(http.StatusBadRequest, "no file part received")
	}
}
