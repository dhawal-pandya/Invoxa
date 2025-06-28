package handlers

import (
	"net/http"
	"strconv"

	"invoxa/database"
	"invoxa/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CreateOrganizationRequest struct {
	Name         string `json:"name" binding:"required"`
	BillingEmail string `json:"billing_email" binding:"required,email"`
}

func CreateOrganization(c *gin.Context) {
	var req CreateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existingOrg models.Organization
	if err := database.DB.Where("name = ?", req.Name).First(&existingOrg).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Organization with this name already exists"})
		return
	} else if err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check for existing organization"})
		return
	}

	organization := models.Organization{
		Name:         req.Name,
		BillingEmail: req.BillingEmail,
	}

	if err := database.DB.Create(&organization).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create organization"})
		return
	}

	c.JSON(http.StatusCreated, organization)
}

type OrgSummaryResponse struct {
	OrganizationName string           `json:"organization_name"`
	BillingEmail     string           `json:"billing_email"`
	TotalUsers       int64            `json:"total_users"`
	TotalInvoices    int64            `json:"total_invoices"`
	TotalRevenue     float64          `json:"total_revenue"`
	LatestInvoices   []models.Invoice `json:"latest_invoices"`
	RecentPayments   []models.Payment `json:"recent_payments"`
}

func GetOrgSummary(c *gin.Context) {
	orgIDStr := c.Param("id")
	orgID, err := strconv.ParseUint(orgIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	callerOrganizationID := c.GetUint64("callerOrganizationID")

	if orgID != callerOrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized: Caller organization ID does not match target organization ID"})
		return
	}

	var organization models.Organization
	if err := database.DB.First(&organization, orgID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve organization"})
		return
	}

	var totalUsers int64
	database.DB.Model(&models.User{}).Where("organization_id = ?", orgID).Count(&totalUsers)

	var invoices []models.Invoice
	database.DB.Where("organization_id = ?", orgID).Find(&invoices)

	var totalRevenue float64
	for _, invoice := range invoices {
		totalRevenue += invoice.Amount
	}

	var latestInvoices []models.Invoice
	database.DB.Where("organization_id = ?", orgID).Order("issue_date desc").Limit(5).Find(&latestInvoices)

	var recentPayments []models.Payment
	database.DB.Joins("JOIN invoices ON payments.invoice_id = invoices.id").Where("invoices.organization_id = ?", orgID).Order("payments.payment_date desc").Limit(5).Find(&recentPayments)

	response := OrgSummaryResponse{
		OrganizationName: organization.Name,
		BillingEmail:     organization.BillingEmail,
		TotalUsers:       totalUsers,
		TotalInvoices:    int64(len(invoices)),
		TotalRevenue:     totalRevenue,
		LatestInvoices:   latestInvoices,
		RecentPayments:   recentPayments,
	}

	c.JSON(http.StatusOK, response)
}
