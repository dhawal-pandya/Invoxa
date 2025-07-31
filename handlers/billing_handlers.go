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
	ctx, span := Tracer.StartSpan(c.Request.Context(), "Subscribe")
	defer span.End()

	var req SubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Manual validation that was in validateSubscriptionRequest
	callerOrganizationID := c.GetUint64("callerOrganizationID")
	if uint(callerOrganizationID) != req.OrganizationID {
		err := errors.New("unauthorized: caller organization id does not match target organization id")
		span.SetError(err.Error(), "")
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	err, plan := validateSubscriptionRequest(ctx, req)
	if err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err, subscription := createSubscription(ctx, req, plan)
	if err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	err, invoice := createInvoice(ctx, req, plan)
	if err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":         "Subscription and initial invoice created successfully",
		"subscription_id": subscription.ID,
		"invoice_id":      invoice.ID,
	})
}

func validateSubscriptionRequest(ctx context.Context, req SubscribeRequest) (error, models.SubscriptionPlan) {
	_, span := Tracer.StartSpan(ctx, "validateSubscriptionRequest")
	defer span.End()

	// The original function was using c.GetUint64, which we can't do here.
	// This validation should be done in the main handler before calling this function.
	// For now, we will proceed without it, assuming the caller has validated.

	var organization models.Organization
	if err := database.DB.WithContext(ctx).First(&organization, req.OrganizationID).Error; err != nil {
		span.SetError(err.Error(), "")
		return errors.New("Organization not found"), models.SubscriptionPlan{}
	}

	var plan models.SubscriptionPlan
	if err := database.DB.WithContext(ctx).Where("id = ? AND organization_id = ?", req.SubscriptionPlanID, req.OrganizationID).First(&plan).Error; err != nil {
		span.SetError(err.Error(), "")
		return errors.New("Subscription plan not found for this organization"), models.SubscriptionPlan{}
	}

	var user models.User
	if err := database.DB.WithContext(ctx).Where("id = ? AND organization_id = ?", req.UserID, req.OrganizationID).First(&user).Error; err != nil {
		span.SetError(err.Error(), "")
		return errors.New("User not found or does not belong to the target organization"), models.SubscriptionPlan{}
	}

	return nil, plan
}

func createSubscription(ctx context.Context, req SubscribeRequest, plan models.SubscriptionPlan) (error, models.Subscription) {
	_, span := Tracer.StartSpan(ctx, "createSubscription")
	defer span.End()

	subscription := models.Subscription{
		OrganizationID:     req.OrganizationID,
		SubscriptionPlanID: plan.ID,
		StartDate:          time.Now(),
		IsActive:           true,
	}
	if err := database.DB.WithContext(ctx).Create(&subscription).Error; err != nil {
		span.SetError(err.Error(), "")
		return errors.New("Failed to create subscription"), models.Subscription{}
	}
	return nil, subscription
}

func createInvoice(ctx context.Context, req SubscribeRequest, plan models.SubscriptionPlan) (error, models.Invoice) {
	_, span := Tracer.StartSpan(ctx, "createInvoice")
	defer span.End()

	invoice := models.Invoice{
		OrganizationID: req.OrganizationID,
		UserID:         req.UserID,
		Amount:         plan.Price,
		Currency:       plan.Currency,
		IssueDate:      time.Now(),
		DueDate:        time.Now().AddDate(0, 1, 0), // due in 1 month for monthly plans
		Paid:           false,
	}
	if err := database.DB.WithContext(ctx).Create(&invoice).Error; err != nil {
		span.SetError(err.Error(), "")
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
	_, span := Tracer.StartSpan(c.Request.Context(), "PayInvoice")
	defer span.End()

	var req PayInvoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	span.SetAttributes(map[string]interface{}{
		"invoice_id": req.InvoiceID,
		"user_id":    req.UserID,
		"amount":     req.Amount,
	})

	callerOrganizationID := c.GetUint64("callerOrganizationID")

	var invoice models.Invoice
	if err := database.DB.Preload("Organization").First(&invoice, req.InvoiceID).Error; err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
		return
	}

	if invoice.OrganizationID != uint(callerOrganizationID) {
		err := errors.New("unauthorized: invoice does not belong to the caller's organization")
		span.SetError(err.Error(), "")
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if invoice.Paid {
		err := errors.New("invoice is already paid")
		span.SetError(err.Error(), "")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Amount < invoice.Amount {
		err := errors.New("payment amount is less than invoice amount")
		span.SetError(err.Error(), "")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payment amount is less than invoice amount. Partial payments not supported in this version."})
		return
	}

	var user models.User
	if err := database.DB.Where("id = ? AND organization_id = ?", req.UserID, invoice.OrganizationID).First(&user).Error; err != nil {
		span.SetError(err.Error(), "")
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
		span.SetError(err.Error(), "")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payment record"})
		return
	}

	invoice.Paid = true
	if err := database.DB.Save(&invoice).Error; err != nil {
		span.SetError(err.Error(), "")
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
	_, span := Tracer.StartSpan(c.Request.Context(), "UpgradePlan")
	defer span.End()

	var req UpgradePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	span.SetAttributes(map[string]interface{}{
		"organization_id":          req.OrganizationID,
		"new_subscription_plan_id": req.NewSubscriptionPlanID,
		"user_id":                  req.UserID,
	})

	callerOrganizationID := c.GetUint64("callerOrganizationID")

	if uint(callerOrganizationID) != req.OrganizationID {
		err := errors.New("unauthorized: caller organization id does not match target organization id")
		span.SetError(err.Error(), "")
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var organization models.Organization
	if err := database.DB.Preload("Subscriptions").Preload("Subscriptions.SubscriptionPlan").First(&organization, req.OrganizationID).Error; err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	var newPlan models.SubscriptionPlan
	if err := database.DB.Where("id = ? AND organization_id = ?", req.NewSubscriptionPlanID, req.OrganizationID).First(&newPlan).Error; err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusNotFound, gin.H{"error": "New subscription plan not found for this organization"})
		return
	}

	var user models.User
	if err := database.DB.Where("id = ? AND organization_id = ?", req.UserID, req.OrganizationID).First(&user).Error; err != nil {
		span.SetError(err.Error(), "")
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
		err := errors.New("no active subscription found for this organization")
		span.SetError(err.Error(), "")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if currentSubscription.SubscriptionPlanID == req.NewSubscriptionPlanID {
		err := errors.New("cannot upgrade to the same plan")
		span.SetError(err.Error(), "")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var currentPlan models.SubscriptionPlan
	if err := database.DB.First(&currentPlan, currentSubscription.SubscriptionPlanID).Error; err != nil {
		span.SetError(err.Error(), "")
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
		span.SetError(err.Error(), "")
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
		span.SetError(err.Error(), "")
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
		span.SetError(err.Error(), "")
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
	_, span := Tracer.StartSpan(c.Request.Context(), "GetInvoice")
	defer span.End()

	invoiceIDStr := c.Param("id")
	invoiceID, err := strconv.ParseUint(invoiceIDStr, 10, 64)
	if err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid invoice ID"})
		return
	}
	span.SetAttributes(map[string]interface{}{"invoice_id": invoiceID})

	callerOrganizationID := c.GetUint64("callerOrganizationID")

	var invoice models.Invoice
	if err := database.DB.Preload("Organization").Preload("User").First(&invoice, invoiceID).Error; err != nil {
		span.SetError(err.Error(), "")
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve invoice"})
		return
	}

	if invoice.OrganizationID != uint(callerOrganizationID) {
		err := errors.New("unauthorized: invoice does not belong to the caller's organization")
		span.SetError(err.Error(), "")
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
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
	_, span := Tracer.StartSpan(c.Request.Context(), "Refund")
	defer span.End()

	var req RefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	span.SetAttributes(map[string]interface{}{
		"invoice_id": req.InvoiceID,
		"payment_id": req.PaymentID,
		"amount":     req.Amount,
	})

	callerOrganizationID := c.GetUint64("callerOrganizationID")

	var invoice models.Invoice
	if err := database.DB.First(&invoice, req.InvoiceID).Error; err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
		return
	}

	if invoice.OrganizationID != uint(callerOrganizationID) {
		err := errors.New("unauthorized: invoice does not belong to the caller's organization")
		span.SetError(err.Error(), "")
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var payment models.Payment
	if err := database.DB.Where("id = ? AND invoice_id = ?", req.PaymentID, req.InvoiceID).First(&payment).Error; err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusNotFound, gin.H{"error": "Payment not found or does not belong to the specified invoice"})
		return
	}

	var user models.User
	if err := database.DB.Where("id = ? AND organization_id = ?", req.UserID, invoice.OrganizationID).First(&user).Error; err != nil {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found or does not belong to the organization associated with the invoice"})
		return
	}

	var existingRefund models.Refund
	if err := database.DB.Where("payment_id = ? AND transaction_id = ?", req.PaymentID, req.TransactionID).First(&existingRefund).Error; err == nil {
		err = errors.New("a refund with this transaction id already exists for this payment")
		span.SetError(err.Error(), "")
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	} else if err != gorm.ErrRecordNotFound {
		span.SetError(err.Error(), "")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check for existing refund"})
		return
	}

	if req.Amount > payment.Amount {
		err := errors.New("refund amount cannot exceed original payment amount")
		span.SetError(err.Error(), "")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		span.SetError(err.Error(), "")
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

