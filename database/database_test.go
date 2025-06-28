package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDatabaseConnection tests the connection to the database.
// NOTE: This is an integration test and requires a running PostgreSQL database
// with the DSN specified in the ConnectDatabase function.
func TestDatabaseConnection(t *testing.T) {
	// Attempt to connect to the database
	ConnectDatabase()

	// Check if the database connection object is not nil
	assert.NotNil(t, DB, "Database connection should not be nil")

	// Check if we can ping the database
	sqlDB, err := DB.DB()
	assert.NoError(t, err, "Failed to get underlying sql.DB")

	err = sqlDB.Ping()
	assert.NoError(t, err, "Failed to ping database")
}

// TestClearDBAndMigrate tests the ClearDBAndMigrate function.
// NOTE: This is a destructive test. It will drop all tables and re-migrate.
// It should only be run in a test environment.
func TestClearDBAndMigrate(t *testing.T) {
	// Connect to the database first
	ConnectDatabase()
	assert.NotNil(t, DB, "Database connection should not be nil")

	// Run the function to clear and migrate the database
	err := ClearDBAndMigrate()
	assert.NoError(t, err, "ClearDBAndMigrate should not return an error")
}
