package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"parcel/internal/auth"
	"parcel/internal/storage"
)

type registerRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=72"` // bcrypt limit is 72 bytes
}

type loginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (a *app) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(
			c,
			http.StatusBadRequest,
			"name, a valid email and a password of 8-72 characters are required",
		)
		return
	}

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(req.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "internal error")
		return
	}

	user := &storage.User{
		Name:         strings.TrimSpace(req.Name),
		Email:        strings.ToLower(strings.TrimSpace(req.Email)),
		PasswordHash: string(hash),
	}

	if err := a.users.Create(c.Request.Context(), user); err != nil {
		if errors.Is(err, storage.ErrEmailTaken) {
			respondError(c, http.StatusConflict, err.Error())
			return
		}

		respondError(c, http.StatusInternalServerError, "could not create user")
		return
	}

	a.respondWithToken(c, http.StatusCreated, user)
}

func (a *app) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "email and password are required")
		return
	}

	user, err := a.users.GetByEmail(
		c.Request.Context(),
		strings.ToLower(strings.TrimSpace(req.Email)),
	)
	if err != nil && !errors.Is(err, storage.ErrUserNotFound) {
		respondError(c, http.StatusInternalServerError, "internal error")
		return
	}

	// Same error for "no such user" and "wrong password" so emails can't be probed.
	if user == nil ||
		bcrypt.CompareHashAndPassword(
			[]byte(user.PasswordHash),
			[]byte(req.Password),
		) != nil {
		respondError(c, http.StatusUnauthorized, "invalid email or password")
		return
	}

	a.respondWithToken(c, http.StatusOK, user)
}

func (a *app) respondWithToken(c *gin.Context, status int, u *storage.User) {
	token, err := auth.NewToken(a.jwtSecret, u.ID, u.Role)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not create token")
		return
	}

	c.JSON(status, gin.H{
		"token": token,
		"user": gin.H{
			"id":    u.ID,
			"name":  u.Name,
			"email": u.Email,
			"role":  u.Role,
		},
	})
}
