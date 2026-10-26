package main

import "github.com/gin-gonic/gin"

func respondError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{
		"error": msg,
	})
}
