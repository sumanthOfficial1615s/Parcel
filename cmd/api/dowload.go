package main

import (
	"errors"
	"fmt"
	"net/http"

	"parcel/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
)

func (a *app) Download(c *gin.Context) {
	var request storage.DownloadRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		RespondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	file, err := a.storage.Postgres.GetForDownload(
		c,
		&request,
	)

	if err != nil {
		if errors.Is(err, storage.ErrFileExpired) && file != nil {
			err := a.storage.MinIO.RemoveObject(
				c,
				a.storage.Bucket,
				file.StorageKey,
				minio.RemoveObjectOptions{},
			)
			if err != nil {
				RespondError(
					c,
					http.StatusInternalServerError,
					"failed to delete expired file",
				)
				return
			}

			RespondError(
				c,
				http.StatusGone,
				"file has expired",
			)
			return
		}

		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	object, err := a.storage.MinIO.GetObject(
		c,
		a.storage.Bucket,
		file.StorageKey,
		minio.GetObjectOptions{},
	)
	if err != nil {
		RespondError(
			c,
			http.StatusInternalServerError,
			"failed to get file",
		)
		return
	}
	defer object.Close()

	info, err := object.Stat()
	if err != nil {
		RespondError(
			c,
			http.StatusInternalServerError,
			"failed to get file information",
		)
		return
	}

	c.Header("Accept-Ranges", "bytes")
	c.Header(
		"Content-Disposition",
		fmt.Sprintf(
			`attachment; filename="%s"`,
			file.OriginalName,
		),
	)
	c.Header("Content-Type", "application/octet-stream")

	http.ServeContent(
		c.Writer,
		c.Request,
		file.OriginalName,
		info.LastModified,
		object,
	)
}
