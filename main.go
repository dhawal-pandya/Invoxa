package main

import (
	"invoxa/database"
	"invoxa/handlers"
	"log"
	"os"

	tracer "github.com/dhawal-pandya/aeonis/packages/tracer-sdk/go"
	"github.com/gin-gonic/gin"
)

func main() {
	apiKey := os.Getenv("AEONIS_API_KEY")
	if apiKey == "" {
		log.Fatal("AEONIS_API_KEY environment variable not set")
	}
	log.Printf("Initializing tracer with hardcoded API Key: %s", apiKey)

	aeonisTracer := tracer.NewTracer(
		"invoxa-test",
		"http://localhost:8000/v1/traces",
		apiKey,
		tracer.NewPIISanitizer(),
	)
	defer aeonisTracer.Shutdown()

	handlers.SetTracer(aeonisTracer)

	database.ConnectDatabase()

	r := gin.Default()

	// Middleware to create the root span for each request.
	r.Use(func(c *gin.Context) {
		ctx, span := aeonisTracer.StartSpan(c.Request.Context(), c.Request.URL.Path)
		defer span.End()

		span.SetAttributes(map[string]interface{}{
			"http.method":     c.Request.Method,
			"http.url":        c.Request.URL.String(),
			"http.client_ip":  c.ClientIP(),
			"http.user_agent": c.Request.UserAgent(),
		})

		c.Request = c.Request.WithContext(ctx)
		c.Next()

		span.SetAttributes(map[string]interface{}{
			"http.status_code": c.Writer.Status(),
		})
	})

	authRequired := r.Group("/")
	authRequired.Use(handlers.AuthMiddleware())
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

	log.Fatal(r.Run(":8081"))
}