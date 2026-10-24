package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"golang.org/x/crypto/bcrypt"

	"parcel/internal/storage"
)

func RespondError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{
		"error": msg,
	})
}

func (a *app) Upload(c *gin.Context) {
	reader, err := c.Request.MultipartReader()
	if err != nil {
		RespondError(
			c,
			http.StatusBadRequest,
			"expected multipart form",
		)
		return
	}

	file := &storage.File{}

	passwordSet := false
	expiresAtSet := false
	fileHandled := false

	ctx := c.Request.Context()

	for {
		part, err := reader.NextPart()

		if err == io.EOF {
			break
		}

		if err != nil {
			RespondError(
				c,
				http.StatusBadRequest,
				"malformed multipart",
			)
			return
		}

		switch part.FormName() {

		case "password":
			buf, err := io.ReadAll(part)
			if err != nil {
				RespondError(
					c,
					http.StatusBadRequest,
					"could not read password field",
				)
				return
			}

			hashedPassword, err := bcrypt.GenerateFromPassword(
				buf,
				bcrypt.DefaultCost,
			)
			if err != nil {
				RespondError(
					c,
					http.StatusInternalServerError,
					"internal error",
				)
				return
			}

			file.PasswordHash = string(hashedPassword)
			passwordSet = true

		case "expires_at":
			buf, err := io.ReadAll(part)
			if err != nil {
				RespondError(
					c,
					http.StatusBadRequest,
					"could not read expires_at field",
				)
				return
			}

			parsed, err := time.Parse(
				time.RFC3339,
				string(buf),
			)
			if err != nil {
				RespondError(
					c,
					http.StatusBadRequest,
					"invalid expires_at format",
				)
				return
			}

			file.ExpiresAt = parsed
			expiresAtSet = true

		case "uploader_name":
			buf, err := io.ReadAll(part)
			if err != nil {
				RespondError(
					c,
					http.StatusBadRequest,
					"could not read uploader name",
				)
				return
			}

			file.UploaderName = string(buf)

		default:
			if !passwordSet {
				RespondError(
					c,
					http.StatusBadRequest,
					"password must be sent before the file part",
				)
				return
			}

			if !expiresAtSet {
				RespondError(
					c,
					http.StatusBadRequest,
					"expires_at must be sent before the file part",
				)
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

				_, err := io.Copy(mw, part)
				if err != nil {
					pw.CloseWithError(err)
				}
			}()

			storageKey := uuid.New().String()

			info, err := a.storage.MinIO.PutObject(
				ctx,
				a.storage.Bucket,
				storageKey,
				pr,
				-1,
				minio.PutObjectOptions{},
			)

			if err != nil {
				RespondError(
					c,
					http.StatusInternalServerError,
					"upload failed",
				)
				return
			}

			file.OriginalName = part.FileName()
			file.StorageKey = info.Key
			file.SizeBytes = info.Size
			file.ContentHash = hex.EncodeToString(
				hasher.Sum(nil),
			)

			if err := a.storage.Postgres.Upload(
				ctx,
				file,
			); err != nil {
				RespondError(
					c,
					http.StatusInternalServerError,
					"could not save file record",
				)
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"id":         file.ID,
				"filename":   file.OriginalName,
				"size":       file.SizeBytes,
				"sha256":     file.ContentHash,
				"expires_at": file.ExpiresAt,
				"secret":     file.StorageKey,
			})

			return
		}
	}

	if !fileHandled {
		RespondError(
			c,
			http.StatusBadRequest,
			"no file part received",
		)
	}
}
