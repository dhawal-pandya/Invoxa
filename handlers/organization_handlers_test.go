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

func setupOrgTestDB(t *testing.T) (*gorm.DB, *models.Organization, *models.User) {
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

func TestCreateOrganization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, _, user := setupOrgTestDB(t)

	r := gin.Default()
	r.Use(AuthMiddleware())
	r.POST("/organizations", CreateOrganization)

	orgReq := CreateOrganizationRequest{
		Name:         "New Test Org",
		BillingEmail: "newbilling@test.org",
	}

	jsonValue, _ := json.Marshal(orgReq)
	req, _ := http.NewRequest("POST", fmt.Sprintf("/organizations?caller_user_id=%d&caller_organization_id=%d", user.ID, user.OrganizationID), bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "Organization created successfully")

	var org models.Organization
	database.DB.First(&org, "name = ?", "New Test Org")
	assert.Equal(t, "New Test Org", org.Name)
}

func TestGetOrgSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, org, user := setupOrgTestDB(t)

	r := gin.Default()
	r.Use(AuthMiddleware())
	r.GET("/org/:id/summary", GetOrgSummary)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/org/%d/summary?caller_user_id=%d&caller_organization_id=%d", org.ID, user.ID, org.ID), nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var summary OrgSummaryResponse
	err := json.Unmarshal(w.Body.Bytes(), &summary)
	assert.NoError(t, err)

	assert.Equal(t, org.Name, summary.OrganizationName)
	assert.Equal(t, int64(1), summary.TotalUsers)
}
