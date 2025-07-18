package main

import (
	"log"

	"invoxa/database"
	"invoxa/handlers"

	sdktracer "github.com/dhawal-pandya/aeonis/packages/tracer-sdk/go"
	"github.com/gin-gonic/gin"
)

func main() {
	exporter := sdktracer.NewHTTPExporter("http://localhost:8000/v1/traces", "YR48BH-GPY9ISJ0820Zs-y4h0z-xqy6gaoDFJc9I3AI")
	sanitizer := sdktracer.NewPIISanitizer()
	handlers.Tracer = sdktracer.NewTracerWithExporter("invoxa-test", exporter, sanitizer)

	database.ConnectDatabase()

	r := gin.Default()

	r.Use(func(c *gin.Context) {
		ctx, span := handlers.Tracer.StartSpan(c.Request.Context(), c.Request.URL.Path)
		defer span.End()

		span.SetAttributes(map[string]interface{}{
			"http.method": c.Request.Method,
			"http.url":    c.Request.URL.String(),
			"http.client_ip": c.ClientIP(),
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
