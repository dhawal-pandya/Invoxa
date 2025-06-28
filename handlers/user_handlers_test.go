package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"invoxa/database"
	"invoxa/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupUserTestDB(t *testing.T) (*gorm.DB, *models.Organization, *models.User) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	assert.NoError(t, err)
	database.DB = db

	err = db.AutoMigrate(
		&models.User{},
		&models.Organization{},
		&models.SubscriptionPlan{},
		&models.Subscription{},
		&models.Invoice{},
		&models.Payment{},
		&models.Refund{},
	)
	assert.NoError(t, err)

	// Create a test organization
	org := models.Organization{Name: "Test Org", BillingEmail: "billing@test.org"}
	err = db.Create(&org).Error
	assert.NoError(t, err)

	// Create a test user for the organization
	user := models.User{Username: "testuser", Email: "test@test.org", PasswordHash: "hash", OrganizationID: org.ID}
	err = db.Create(&user).Error
	assert.NoError(t, err)

	return db, &org, &user
}

func TestCreateUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, org, user := setupUserTestDB(t)

	r := gin.Default()
	r.Use(AuthMiddleware())
	r.POST("/users", CreateUser)

	userReq := CreateUserRequest{
		Username:       "newuser",
		Email:          "newuser@test.org",
		Password:       "password",
		OrganizationID: org.ID,
	}

	jsonValue, _ := json.Marshal(userReq)
	req, _ := http.NewRequest("POST", fmt.Sprintf("/users?caller_user_id=%d&caller_organization_id=%d", user.ID, org.ID), bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "User created successfully")

	var newUser models.User
	database.DB.First(&newUser, "username = ?", "newuser")
	assert.Equal(t, "newuser", newUser.Username)
}

func TestGetUserSubscriptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, org, user := setupUserTestDB(t)

	r := gin.Default()
	r.Use(AuthMiddleware())
	r.GET("/user/:id/subscriptions", GetUserSubscriptions)

	// Create a subscription for the user's organization
	plan := models.SubscriptionPlan{Name: "Test Plan", Price: 10, Currency: "USD", Interval: "monthly", OrganizationID: org.ID}
	database.DB.Create(&plan)
	subscription := models.Subscription{OrganizationID: org.ID, SubscriptionPlanID: plan.ID, IsActive: true}
	database.DB.Create(&subscription)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/user/%d/subscriptions?caller_user_id=%d&caller_organization_id=%d", user.ID, user.ID, org.ID), nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var subscriptions []models.Subscription
	err := json.Unmarshal(w.Body.Bytes(), &subscriptions)
	assert.NoError(t, err)

	assert.Len(t, subscriptions, 1)
	assert.Equal(t, subscription.ID, subscriptions[0].ID)
}
