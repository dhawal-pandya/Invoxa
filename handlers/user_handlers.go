package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"invoxa/database"
	"invoxa/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type CreateUserRequest struct {
	Username       string `json:"username" binding:"required"`
	Email          string `json:"email" binding:"required,email"`
	Password       string `json:"password" binding:"required,min=6"`
	OrganizationID uint   `json:"organization_id" binding:"required"`
}

func CreateUser(c *gin.Context) {
	_, span := Tracer.StartSpan(c.Request.Context(), "CreateUser")
	defer span.End()
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	span.SetAttributes(map[string]interface{}{"username": req.Username, "email": req.Email})

	var organization models.Organization
	if err := database.DB.First(&organization, req.OrganizationID).Error; err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusNotFound, gin.H{"error": "Target organization not found"})
		return
	}

	var existingUser models.User
	if err := database.DB.Where("username = ? AND organization_id = ?", req.Username, req.OrganizationID).First(&existingUser).Error; err == nil {
		err = errors.New("username already exists for this organization")
		span.SetError(err.Error(), "")
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	} else if err != gorm.ErrRecordNotFound {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check for existing username"})
		return
	}

	var existingEmailUser models.User
	if err := database.DB.Where("email = ?", req.Email).First(&existingEmailUser).Error; err == nil {
		err = errors.New("email address already in use")
		span.SetError(err.Error(), "")
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	} else if err != gorm.ErrRecordNotFound {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check for existing email"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := models.User{
		Username:       req.Username,
		Email:          req.Email,
		PasswordHash:   string(hashedPassword),
		OrganizationID: req.OrganizationID,
	}

	if err := database.DB.Create(&user).Error; err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User created successfully", "user_id": user.ID})
}

func GetUserSubscriptions(c *gin.Context) {
	_, span := Tracer.StartSpan(c.Request.Context(), "GetUserSubscriptions")
	defer span.End()

	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	span.SetAttributes(map[string]interface{}{"user_id": userID})

	callerUserID := c.GetUint64("callerUserID")
	callerOrganizationID := c.GetUint64("callerOrganizationID")

	if userID != callerUserID {
		err := errors.New("unauthorized: you can only view your own subscriptions")
		span.SetError(err.Error(), "")
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := database.DB.Preload("Organization").First(&user, userID).Error; err != nil {
		span.SetError(err.Error(), "")
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user"})
		return
	}

	if user.OrganizationID != uint(callerOrganizationID) {
		err := errors.New("unauthorized: user does not belong to the calling organization")
		span.SetError(err.Error(), "")
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var subscriptions []models.Subscription
	if err := database.DB.Where("organization_id = ?", user.OrganizationID).Preload("SubscriptionPlan").Find(&subscriptions).Error; err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve subscriptions for user's organization"})
		return
	}

	c.JSON(http.StatusOK, subscriptions)
}
