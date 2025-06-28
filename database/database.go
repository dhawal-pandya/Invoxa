package database

import (
	"fmt"
	"log"

	"invoxa/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDatabase() {
	dsn := "host=localhost user=dhawalpandya password='' dbname=invoxadb port=5432 sslmode=disable TimeZone=Asia/Shanghai"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	DB = db

	// Auto-migrate models
	log.Println("Running database migrations...")
	err = DB.AutoMigrate(
		&models.User{},
		&models.Organization{},
		&models.SubscriptionPlan{},
		&models.Subscription{},
		&models.Invoice{},
		&models.Payment{},
		&models.Refund{},
	)
	if err != nil {
		log.Fatalf("Failed to auto migrate database: %v", err)
	}

	log.Println("Database migration completed!")
}

// ClearDBAndMigrate drops all tables and re-runs migrations.
// This is primarily for development/testing purposes.
func ClearDBAndMigrate() error {
	log.Println("Clearing database...")
	// Drop all tables
	err := DB.Migrator().DropTable(
		&models.User{},
		&models.Organization{},
		&models.SubscriptionPlan{},
		&models.Subscription{},
		&models.Invoice{},
		&models.Payment{},
		&models.Refund{},
	)
	if err != nil {
		log.Printf("Failed to drop tables: %v", err)
		return fmt.Errorf("failed to drop tables: %w", err)
	}

	log.Println("Database cleared. Running migrations again...")
	// Re-run migrations
	err = DB.AutoMigrate(
		&models.User{},
		&models.Organization{},
		&models.SubscriptionPlan{},
		&models.Subscription{},
		&models.Invoice{},
		&models.Payment{},
		&models.Refund{},
	)
	if err != nil {
		log.Printf("Failed to re-migrate database: %v", err)
		return fmt.Errorf("failed to re-migrate database: %w", err)
	}
	log.Println("Database re-migrated successfully!")
	return nil
}
