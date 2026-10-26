package main

import (
	"errors"
	"mime"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"parcel/internal/auth"
	"parcel/internal/storage"
)

// downloadLinkTTL is how long a link from POST /download keeps working.
// An interrupted download can only be resumed while its link is valid.
const downloadLinkTTL = time.Hour

// Download checks the file password and returns a short-lived download link.
// The password only ever travels in this POST body, never in a URL.
func (a *app) Download(c *gin.Context) {
	var request storage.DownloadRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	file, err := a.files.GetForDownload(c.Request.Context(), &request)
	if err != nil {
		a.respondDownloadError(c, file, err, http.StatusBadRequest)
		return
	}

	token, err := auth.NewDownloadToken(a.jwtSecret, file.ID, downloadLinkTTL)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not create download link", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"url":        "/download/" + token,
		"filename":   file.OriginalName,
		"size":       file.SizeBytes,
		"expires_in": int(downloadLinkTTL.Seconds()),
	})
}

// DownloadFile streams the file for a valid download link.
// http.ServeContent handles Range / If-Range requests (206 Partial Content),
// which is what makes downloads resumable.
func (a *app) DownloadFile(c *gin.Context) {
	fileID, err := auth.ParseDownloadToken(a.jwtSecret, c.Param("token"))
	if err != nil {
		respondError(c, http.StatusUnauthorized, err.Error())
		return
	}

	ctx := c.Request.Context()

	file, err := a.files.GetAvailable(ctx, fileID)
	if err != nil {
		a.respondDownloadError(c, file, err, http.StatusNotFound)
		return
	}

	object, err := a.objects.Get(ctx, file.StorageKey)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to get file", err)
		return
	}
	defer object.Close()

	info, err := object.Stat()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to get file information", err)
		return
	}

	disposition := mime.FormatMediaType(
		"attachment",
		map[string]string{"filename": file.OriginalName},
	)
	if disposition == "" {
		disposition = "attachment"
	}

	c.Header("Accept-Ranges", "bytes")
	// A stable validator lets the browser check the file didn't change
	// before it resumes (ServeContent uses it for If-Range).
	c.Header("ETag", `"`+file.ContentHash+`"`)
	c.Header("Content-Disposition", disposition)
	c.Header("Content-Type", "application/octet-stream")

	http.ServeContent(
		c.Writer,
		c.Request,
		file.OriginalName,
		info.LastModified,
		object,
	)
}

// respondDownloadError maps a store error to a response. Expired files also
// get their stored object removed.
func (a *app) respondDownloadError(
	c *gin.Context,
	file *storage.File,
	err error,
	fallbackStatus int,
) {
	if errors.Is(err, storage.ErrFileExpired) && file != nil {
		if err := a.objects.Remove(c.Request.Context(), file.StorageKey); err != nil {
			respondError(c, http.StatusInternalServerError, "failed to delete expired file", err)
			return
		}

		respondError(c, http.StatusGone, "file has expired")
		return
	}

	respondError(c, fallbackStatus, err.Error())
}
