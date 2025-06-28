package handlers

import (
	"net/http"

	"invoxa/database"

	"github.com/gin-gonic/gin"
)

func ClearDatabase(c *gin.Context) {

	err := database.ClearDBAndMigrate()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear and migrate database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Database cleared and migrated successfully"})
}
