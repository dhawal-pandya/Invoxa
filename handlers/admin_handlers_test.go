package handlers

import (
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

func setupTestDB(t *testing.T) {
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
}

func TestClearDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupTestDB(t)

	r := gin.Default()
	r.POST("/admin/clear_db", ClearDatabase)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/clear_db", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Database cleared and migrated successfully")
}
