package main

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"parcel/internal/auth"
	"parcel/internal/storage"
)

// ListMyFiles returns the files uploaded by the logged-in user.
func (a *app) ListMyFiles(c *gin.Context) {
	files, err := a.files.ListByOwner(
		c.Request.Context(),
		auth.UserID(c),
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not list files")
		return
	}

	c.JSON(http.StatusOK, gin.H{"files": files})
}

// DeleteFile lets an owner delete their own file; admins can delete any file.
func (a *app) DeleteFile(c *gin.Context) {
	id := c.Param("id")

	if _, err := uuid.Parse(id); err != nil {
		respondError(c, http.StatusNotFound, "file not found")
		return
	}

	ctx := c.Request.Context()

	storageKey, err := a.files.DeleteOwned(
		ctx,
		id,
		auth.UserID(c),
		auth.Role(c) == "admin",
	)
	if err != nil {
		if errors.Is(err, storage.ErrFileNotFound) {
			// Same response whether it doesn't exist or isn't yours.
			respondError(c, http.StatusNotFound, "file not found")
			return
		}

		respondError(c, http.StatusInternalServerError, "could not delete file")
		return
	}

	if err := a.objects.Remove(ctx, storageKey); err != nil {
		respondError(
			c,
			http.StatusInternalServerError,
			"record deleted but object removal failed",
		)
		return
	}

	c.Status(http.StatusNoContent)
}
