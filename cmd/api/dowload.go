package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"parcel/internal/storage"
)

func (a *app) Download(c *gin.Context) {
	var request storage.DownloadRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	ctx := c.Request.Context()

	file, err := a.files.GetForDownload(ctx, &request)
	if err != nil {
		if errors.Is(err, storage.ErrFileExpired) && file != nil {
			if err := a.objects.Remove(ctx, file.StorageKey); err != nil {
				respondError(
					c,
					http.StatusInternalServerError,
					"failed to delete expired file",
				)
				return
			}

			respondError(c, http.StatusGone, "file has expired")
			return
		}

		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	object, err := a.objects.Get(ctx, file.StorageKey)
	if err != nil {
		respondError(
			c,
			http.StatusInternalServerError,
			"failed to get file",
		)
		return
	}
	defer object.Close()

	info, err := object.Stat()
	if err != nil {
		respondError(
			c,
			http.StatusInternalServerError,
			"failed to get file information",
		)
		return
	}

	c.Header("Accept-Ranges", "bytes")
	c.Header(
		"Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, file.OriginalName),
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
