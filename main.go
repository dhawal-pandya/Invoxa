package main

import (
	"log"

	"invoxa/database"
	"invoxa/handlers"

	"github.com/gin-gonic/gin"
)

func main() {
	database.ConnectDatabase()

	r := gin.Default()

	authMiddleware := handlers.AuthMiddleware()

	authRequired := r.Group("/")
	authRequired.Use(authMiddleware)
	{
		authRequired.POST("/subscribe", handlers.Subscribe)
		authRequired.POST("/pay_invoice", handlers.PayInvoice)
		authRequired.POST("/upgrade_plan", handlers.UpgradePlan)
		authRequired.GET("/invoice/:id", handlers.GetInvoice)
		authRequired.POST("/refund", handlers.Refund)
		authRequired.GET("/user/:id/subscriptions", handlers.GetUserSubscriptions)
		authRequired.POST("/subscription_plans", handlers.CreateSubscriptionPlan)

		authRequired.GET("org/:id/summary", handlers.GetOrgSummary)
	}
	
	r.POST("/users", handlers.CreateUser)
	r.POST("/organizations", handlers.CreateOrganization)
	r.POST("/admin/clear_db", handlers.ClearDatabase)

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	log.Fatal(r.Run(":8080"))
}