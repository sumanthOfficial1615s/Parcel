package main

import (
	"fmt"
	"net/http"
	"parcel/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
)

func (a *app) Download(c *gin.Context) {
	var user storage.User

	if err := c.ShouldBindJSON(&user); err != nil {
		RespondError(c, http.StatusBadRequest, "invalid credentials")
		return
	}

	err := a.storage.Postgres.Download(c, &user)
	if err != nil {
		if user.Expired {
			err := a.storage.MinIO.RemoveObject(
				c,
				a.storage.Bucket,
				user.StorageKey,
				minio.RemoveObjectOptions{},
			)
			if err != nil {
				RespondError(c, http.StatusInternalServerError, "internal error")
				return
			}
		}

		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	object, err := a.storage.MinIO.GetObject(
		c,
		a.storage.Bucket,
		user.StorageKey,
		minio.GetObjectOptions{},
	)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to get file")
		return
	}
	defer object.Close()

	info, err := object.Stat()
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to get file information")
		return
	}

	c.Header("Accept-Ranges", "bytes")
	c.Header(
		"Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, user.OriginalName),
	)
	c.Header("Content-Type", "application/octet-stream")

	http.ServeContent(
		c.Writer,
		c.Request,
		user.OriginalName,
		info.LastModified,
		object,
	)
}
