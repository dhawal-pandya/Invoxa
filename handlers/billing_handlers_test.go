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

func TestMain(m *testing.M) {
	InitTracerForTests()
	m.Run()
}

func setupBillingTestDB(t *testing.T) (*gorm.DB, *models.Organization, *models.User) {
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

func TestCreateSubscriptionPlan(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, org, user := setupBillingTestDB(t)

	r := gin.Default()
	r.Use(AuthMiddleware())
	r.POST("/subscription_plans", CreateSubscriptionPlan)

	plan := CreateSubscriptionPlanRequest{
		Name:           "Basic Plan",
		Description:    "A basic subscription plan",
		Price:          9.99,
		Currency:       "USD",
		Interval:       "monthly",
		OrganizationID: org.ID,
	}

	jsonValue, _ := json.Marshal(plan)
	req, _ := http.NewRequest("POST", fmt.Sprintf("/subscription_plans?caller_user_id=%d&caller_organization_id=%d", user.ID, org.ID), bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "Subscription plan created successfully")

	// Verify the plan was created in the DB
	var savedPlan models.SubscriptionPlan
	database.DB.First(&savedPlan, "name = ?", "Basic Plan")
	assert.Equal(t, "Basic Plan", savedPlan.Name)
	assert.Equal(t, org.ID, savedPlan.OrganizationID)
}
