package handlers

import (
	"invoxa/database"
	"invoxa/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/users" {
			c.Next()
			return
		}
		callerUserIDStr := c.Query("caller_user_id")
		callerOrganizationIDStr := c.Query("caller_organization_id")

		callerUserID, err := strconv.ParseUint(callerUserIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid caller user ID"})
			c.Abort()
			return
		}

		callerOrganizationID, err := strconv.ParseUint(callerOrganizationIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid caller organization ID"})
			c.Abort()
			return
		}

		var callerUser models.User
		if err := database.DB.Where("id = ? AND organization_id = ?", callerUserID, callerOrganizationID).First(&callerUser).Error; err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized: Caller user not found or does not belong to the calling organization"})
			c.Abort()
			return
		}

		c.Set("callerUserID", callerUserID)
		c.Set("callerOrganizationID", callerOrganizationID)

		c.Next()
	}
}
