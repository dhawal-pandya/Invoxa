package main

import (
	"log"
	"os"
	"os/exec"
	"strings"

	"invoxa/database"
	"invoxa/handlers"

	sdktracer "github.com/dhawal-pandya/aeonis/packages/tracer-sdk/go"
	"github.com/gin-gonic/gin"
)

func main() {
	apiKey := os.Getenv("AEONIS_API_KEY")
	if apiKey == "" {
		log.Fatal("AEONIS_API_KEY environment variable not set")
	}

	// Get the latest commit hash
	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		log.Fatal("Could not get latest commit hash: ", err)
	}
	commitID := strings.TrimSpace(string(out))

	exporter := sdktracer.NewHTTPExporter("http://localhost:8000/v1/traces", apiKey)
	sanitizer := sdktracer.NewPIISanitizer()
	tracer := sdktracer.NewTracerWithExporter("invoxa-test", exporter, sanitizer)
	handlers.SetTracer(tracer)

	database.ConnectDatabase()

	r := gin.Default()

	r.Use(func(c *gin.Context) {
		ctx, span := handlers.Tracer.StartSpan(c.Request.Context(), c.Request.URL.Path)
		defer span.End()

		span.SetAttributes(map[string]interface{}{
			"http.method":    c.Request.Method,
			"http.url":       c.Request.URL.String(),
			"http.client_ip": c.ClientIP(),
			"http.user_agent": c.Request.UserAgent(),
			"code.filepath":  "invoxa-test/main.go",
			"commit_id":      commitID,
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
