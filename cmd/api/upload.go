package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"parcel/internal/auth"
	"parcel/internal/storage"
	"parcel/internal/tasks"
)

func (a *app) Upload(c *gin.Context) {
	reader, err := c.Request.MultipartReader()
	if err != nil {
		respondError(c, http.StatusBadRequest, "expected multipart form")
		return
	}

	file := &storage.File{OwnerID: auth.UserID(c)}

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
			respondError(c, http.StatusBadRequest, "malformed multipart")
			return
		}

		switch part.FormName() {
		case "password":
			buf, err := io.ReadAll(part)
			if err != nil {
				respondError(
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
				respondError(
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
				respondError(
					c,
					http.StatusBadRequest,
					"could not read expires_at field",
				)
				return
			}

			parsed, err := time.Parse(time.RFC3339, string(buf))
			if err != nil {
				respondError(
					c,
					http.StatusBadRequest,
					"invalid expires_at format",
					err,
				)
				return
			}

			if !parsed.After(time.Now()) {
				respondError(
					c,
					http.StatusBadRequest,
					"expires_at must be in the future",
					err,
				)
				return
			}

			file.ExpiresAt = &parsed
			expiresAtSet = true

		case "uploader_name":
			buf, err := io.ReadAll(part)
			if err != nil {
				respondError(
					c,
					http.StatusBadRequest,
					"could not read uploader name",
					err,
				)
				return
			}

			file.UploaderName = string(buf)

		default:
			if !passwordSet {
				respondError(
					c,
					http.StatusBadRequest,
					"password must be sent before the file part",
					err,
				)
				return
			}

			if !expiresAtSet {
				respondError(
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

				if _, err := io.Copy(mw, part); err != nil {
					pw.CloseWithError(err)
				}
			}()

			storageKey := uuid.New().String()

			size, err := a.objects.Put(ctx, storageKey, pr)
			// Unblock the copy goroutine if Put stopped reading early.
			pr.Close()
			if err != nil {
				respondError(
					c,
					http.StatusInternalServerError,
					"upload failed",
				)
				return
			}

			file.OriginalName = part.FileName()
			file.StorageKey = storageKey
			file.SizeBytes = size
			file.ContentHash = hex.EncodeToString(hasher.Sum(nil))

			if err := a.files.Create(ctx, file); err != nil {
				respondError(
					c,
					http.StatusInternalServerError,
					"could not save file record",
					err,
				)
				return
			}

			if err := tasks.ScheduleExpiry(
				ctx,
				a.queue,
				file.ID,
				file.ExpiresAt,
			); err != nil {
				respondError(
					c,
					http.StatusInternalServerError,
					"file saved but expiry scheduling failed",
					err,
				)
				return
			}

			if err := tasks.ScheduleScan(
				ctx,
				a.queue,
				file.ID,
			); err != nil {
				respondError(
					c,
					http.StatusInternalServerError,
					"file saved but scan scheduling failed",
					err,
				)
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"id":         file.ID,
				"filename":   file.OriginalName,
				"size":       file.SizeBytes,
				"sha256":     file.ContentHash,
				"status":     file.Status,
				"expires_at": file.ExpiresAt,
			})

			return
		}
	}

	if !fileHandled {
		respondError(
			c,
			http.StatusBadRequest,
			"no file part received",
			err,
		)
	}
}
