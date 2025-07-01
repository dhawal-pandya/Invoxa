package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"invoxa/database"
	"invoxa/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SubscribeRequest struct {
	OrganizationID     uint `json:"organization_id" binding:"required"`
	SubscriptionPlanID uint `json:"subscription_plan_id" binding:"required"`
	UserID             uint `json:"user_id" binding:"required"`
}

func Subscribe(c *gin.Context) {
	_, span := Tracer.StartSpan(c.Request.Context(), "Subscribe")
	defer span.End()

	var req SubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate the request
	out := Tracer.TraceFunc(c.Request.Context(), validateSubscriptionRequest, c, req)
	if err, ok := out[0].(error); ok && err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	plan := out[1].(models.SubscriptionPlan)

	// Create the subscription
	out = Tracer.TraceFunc(c.Request.Context(), createSubscription, req, plan)
	if err, ok := out[0].(error); ok && err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	subscription := out[1].(models.Subscription)

	// Create the invoice
	out = Tracer.TraceFunc(c.Request.Context(), createInvoice, req, plan)
	if err, ok := out[0].(error); ok && err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	invoice := out[1].(models.Invoice)

	c.JSON(http.StatusCreated, gin.H{
		"message":         "Subscription and initial invoice created successfully",
		"subscription_id": subscription.ID,
		"invoice_id":      invoice.ID,
	})
}

func validateSubscriptionRequest(c *gin.Context, req SubscribeRequest) (error, models.SubscriptionPlan) {
	callerOrganizationID := c.GetUint64("callerOrganizationID")

	if uint(callerOrganizationID) != req.OrganizationID {
		return errors.New("Unauthorized: Caller organization ID does not match target organization ID"), models.SubscriptionPlan{}
	}

	var organization models.Organization
	if err := database.DB.WithContext(c.Request.Context()).First(&organization, req.OrganizationID).Error; err != nil {
		return errors.New("Organization not found"), models.SubscriptionPlan{}
	}

	var plan models.SubscriptionPlan
	if err := database.DB.WithContext(c.Request.Context()).Where("id = ? AND organization_id = ?", req.SubscriptionPlanID, req.OrganizationID).First(&plan).Error; err != nil {
		return errors.New("Subscription plan not found for this organization"), models.SubscriptionPlan{}
	}

	var user models.User
	if err := database.DB.WithContext(c.Request.Context()).Where("id = ? AND organization_id = ?", req.UserID, req.OrganizationID).First(&user).Error; err != nil {
		return errors.New("User not found or does not belong to the target organization"), models.SubscriptionPlan{}
	}

	return nil, plan
}

func createSubscription(req SubscribeRequest, plan models.SubscriptionPlan) (error, models.Subscription) {
	subscription := models.Subscription{
		OrganizationID:     req.OrganizationID,
		SubscriptionPlanID: plan.ID,
		StartDate:          time.Now(),
		IsActive:           true,
	}
	if err := database.DB.WithContext(context.Background()).Create(&subscription).Error; err != nil {
		return errors.New("Failed to create subscription"), models.Subscription{}
	}
	return nil, subscription
}

func createInvoice(req SubscribeRequest, plan models.SubscriptionPlan) (error, models.Invoice) {
	invoice := models.Invoice{
		OrganizationID: req.OrganizationID,
		UserID:         req.UserID,
		Amount:         plan.Price,
		Currency:       plan.Currency,
		IssueDate:      time.Now(),
		DueDate:        time.Now().AddDate(0, 1, 0), // due in 1 month for monthly plans
		Paid:           false,
	}
	if err := database.DB.WithContext(context.Background()).Create(&invoice).Error; err != nil {
		return errors.New("Failed to create initial invoice"), models.Invoice{}
	}
	return nil, invoice
}


type PayInvoiceRequest struct {
	InvoiceID     uint    `json:"invoice_id" binding:"required"`
	UserID        uint    `json:"user_id" binding:"required"`
	Amount        float64 `json:"amount" binding:"required,gt=0"`
	Currency      string  `json:"currency" binding:"required"`
	TransactionID string  `json:"transaction_id" binding:"required"`
	PaymentMethod string  `json:"payment_method" binding:"required"`
}

func PayInvoice(c *gin.Context) {
	var req PayInvoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	callerOrganizationID := c.GetUint64("callerOrganizationID")

	var invoice models.Invoice
	if err := database.DB.Preload("Organization").First(&invoice, req.InvoiceID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
		return
	}

	if invoice.OrganizationID != uint(callerOrganizationID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized: Invoice does not belong to the caller's organization"})
		return
	}

	if invoice.Paid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invoice is already paid"})
		return
	}

	if req.Amount < invoice.Amount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payment amount is less than invoice amount. Partial payments not supported in this version."})
		return
	}

	var user models.User
	if err := database.DB.Where("id = ? AND organization_id = ?", req.UserID, invoice.OrganizationID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found or does not belong to the organization associated with the invoice"})
		return
	}

	payment := models.Payment{
		InvoiceID:     req.InvoiceID,
		UserID:        req.UserID,
		Amount:        req.Amount,
		Currency:      req.Currency,
		PaymentDate:   time.Now(),
		TransactionID: req.TransactionID,
		PaymentMethod: req.PaymentMethod,
	}

	if err := database.DB.Create(&payment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payment record"})
		return
	}

	invoice.Paid = true
	if err := database.DB.Save(&invoice).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update invoice status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Invoice paid successfully", "payment_id": payment.ID})
}

type UpgradePlanRequest struct {
	OrganizationID        uint `json:"organization_id" binding:"required"`
	NewSubscriptionPlanID uint `json:"new_subscription_plan_id" binding:"required"`
	UserID                uint `json:"user_id" binding:"required"`
}

func UpgradePlan(c *gin.Context) {
	var req UpgradePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	callerOrganizationID := c.GetUint64("callerOrganizationID")

	if uint(callerOrganizationID) != req.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized: Caller organization ID does not match target organization ID"})
		return
	}

	var organization models.Organization
	if err := database.DB.Preload("Subscriptions").Preload("Subscriptions.SubscriptionPlan").First(&organization, req.OrganizationID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	var newPlan models.SubscriptionPlan
	if err := database.DB.Where("id = ? AND organization_id = ?", req.NewSubscriptionPlanID, req.OrganizationID).First(&newPlan).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "New subscription plan not found for this organization"})
		return
	}

	var user models.User
	if err := database.DB.Where("id = ? AND organization_id = ?", req.UserID, req.OrganizationID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found or does not belong to organization"})
		return
	}

	var currentSubscription models.Subscription
	foundActive := false
	for _, sub := range organization.Subscriptions {
		if sub.IsActive {
			currentSubscription = sub
			foundActive = true
			break
		}
	}

	if !foundActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active subscription found for this organization"})
		return
	}

	if currentSubscription.SubscriptionPlanID == req.NewSubscriptionPlanID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot upgrade to the same plan"})
		return
	}

	var currentPlan models.SubscriptionPlan
	if err := database.DB.First(&currentPlan, currentSubscription.SubscriptionPlanID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve current subscription plan details"})
		return
	}

	today := time.Now()
	endOfCurrentMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location()).AddDate(0, 1, 0).Add(-time.Nanosecond)
	daysInMonth := float64(endOfCurrentMonth.Day())
	daysRemaining := float64(endOfCurrentMonth.Day() - today.Day() + 1)

	proratedAmount := (currentPlan.Price / daysInMonth) * daysRemaining

	currentSubscription.EndDate = today
	currentSubscription.IsActive = false
	if err := database.DB.Save(&currentSubscription).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel old subscription"})
		return
	}

	newSubscription := models.Subscription{
		OrganizationID:     req.OrganizationID,
		SubscriptionPlanID: req.NewSubscriptionPlanID,
		StartDate:          today,
		IsActive:           true,
	}
	if err := database.DB.Create(&newSubscription).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create new subscription"})
		return
	}

	invoice := models.Invoice{
		OrganizationID: req.OrganizationID,
		UserID:         req.UserID,
		Amount:         newPlan.Price - proratedAmount, //new plan price minus prorated credit
		Currency:       newPlan.Currency,
		IssueDate:      today,
		DueDate:        today.AddDate(0, 1, 0), // due in 1 month
		Paid:           false,
	}

	if err := database.DB.Create(&invoice).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create prorated invoice"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":             "Plan upgraded successfully",
		"old_subscription_id": currentSubscription.ID,
		"new_subscription_id": newSubscription.ID,
		"prorated_invoice_id": invoice.ID,
		"prorated_amount":     proratedAmount,
	})
}

func GetInvoice(c *gin.Context) {
	invoiceIDStr := c.Param("id")
	invoiceID, err := strconv.ParseUint(invoiceIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid invoice ID"})
		return
	}

	callerOrganizationID := c.GetUint64("callerOrganizationID")

	var invoice models.Invoice
	if err := database.DB.Preload("Organization").Preload("User").First(&invoice, invoiceID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve invoice"})
		return
	}

	if invoice.OrganizationID != uint(callerOrganizationID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized: Invoice does not belong to the caller's organization"})
		return
	}

	c.JSON(http.StatusOK, invoice)
}

type RefundRequest struct {
	InvoiceID     uint    `json:"invoice_id" binding:"required"`
	PaymentID     uint    `json:"payment_id" binding:"required"`
	UserID        uint    `json:"user_id" binding:"required"`
	Amount        float64 `json:"amount" binding:"required,gt=0"`
	Currency      string  `json:"currency" binding:"required"`
	TransactionID string  `json:"transaction_id" binding:"required"`
	Reason        string  `json:"reason" binding:"required"`
}

func Refund(c *gin.Context) {
	var req RefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	callerOrganizationID := c.GetUint64("callerOrganizationID")

	var invoice models.Invoice
	if err := database.DB.First(&invoice, req.InvoiceID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
		return
	}

	if invoice.OrganizationID != uint(callerOrganizationID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized: Invoice does not belong to the caller's organization"})
		return
	}

	var payment models.Payment
	if err := database.DB.Where("id = ? AND invoice_id = ?", req.PaymentID, req.InvoiceID).First(&payment).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payment not found or does not belong to the specified invoice"})
		return
	}

	var user models.User
	if err := database.DB.Where("id = ? AND organization_id = ?", req.UserID, invoice.OrganizationID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found or does not belong to the organization associated with the invoice"})
		return
	}

	var existingRefund models.Refund
	if err := database.DB.Where("payment_id = ? AND transaction_id = ?", req.PaymentID, req.TransactionID).First(&existingRefund).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "A refund with this transaction ID already exists for this payment"})
		return
	} else if err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check for existing refund"})
		return
	}

	if req.Amount > payment.Amount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Refund amount cannot exceed original payment amount"})
		return
	}

	refund := models.Refund{
		InvoiceID:     req.InvoiceID,
		PaymentID:     req.PaymentID,
		UserID:        req.UserID,
		Amount:        req.Amount,
		Currency:      req.Currency,
		RefundDate:    time.Now(),
		TransactionID: req.TransactionID,
		Reason:        req.Reason,
	}

	if err := database.DB.Create(&refund).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create refund record"})
		return
	}


	c.JSON(http.StatusCreated, gin.H{"message": "Refund created successfully", "refund_id": refund.ID})
}

type CreateSubscriptionPlanRequest struct {
	Name           string  `json:"name" binding:"required"`
	Description    string  `json:"description"`
	Price          float64 `json:"price" binding:"required,gte=0"`
	Currency       string  `json:"currency" binding:"required"`
	Interval       string  `json:"interval" binding:"required"`
	OrganizationID uint    `json:"organization_id" binding:"required"`
}

func CreateSubscriptionPlan(c *gin.Context) {
	_, span := Tracer.StartSpan(c.Request.Context(), "CreateSubscriptionPlan")
	defer span.End()

	var req CreateSubscriptionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	callerOrganizationID := c.GetUint64("callerOrganizationID")

	if uint(callerOrganizationID) != req.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized: Caller organization ID does not match target organization ID"})
		return
	}

	var organization models.Organization
	if err := database.DB.First(&organization, req.OrganizationID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	plan := models.SubscriptionPlan{
		Name:           req.Name,
		Description:    req.Description,
		Price:          req.Price,
		Currency:       req.Currency,
		Interval:       req.Interval,
		OrganizationID: req.OrganizationID,
	}

	var existingPlan models.SubscriptionPlan
	if err := database.DB.Where("name = ? AND organization_id = ?", req.Name, req.OrganizationID).First(&existingPlan).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Subscription plan with this name already exists for this organization"})
		return
	} else if err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check for existing subscription plan"})
		return
	}

	if err := database.DB.Create(&plan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create subscription plan"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Subscription plan created successfully", "plan_id": plan.ID})
}

