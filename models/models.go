package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username       string `gorm:"unique;not null"`
	Email          string `gorm:"unique;not null"`
	PasswordHash   string `gorm:"not null"`
	OrganizationID uint
	Organization   Organization
	Invoices       []Invoice
	Payments       []Payment
	Refunds        []Refund
}

type Organization struct {
	gorm.Model
	Name              string `gorm:"unique;not null"`
	BillingEmail      string `gorm:"not null"`
	Users             []User
	Subscriptions     []Subscription     
	SubscriptionPlans []SubscriptionPlan 
	Invoices          []Invoice
}

type SubscriptionPlan struct {
	gorm.Model
	Name           string `gorm:"not null;uniqueIndex:idx_org_plan_name"` 
	Description    string
	Price          float64 `gorm:"not null"`
	Currency       string  `gorm:"not null;default:'USD'"`
	Interval       string  `gorm:"not null;default:'monthly'"`             // e.g., 'monthly', 'yearly'
	OrganizationID uint    `gorm:"not null;uniqueIndex:idx_org_plan_name"` 
	Organization   Organization
	Subscriptions  []Subscription
}

type Subscription struct {
	gorm.Model
	OrganizationID     uint `gorm:"not null"`
	Organization       Organization
	SubscriptionPlanID uint `gorm:"not null"`
	SubscriptionPlan   SubscriptionPlan
	StartDate          time.Time `gorm:"not null"`
	EndDate            time.Time
	IsActive           bool `gorm:"default:true"`
}

type Invoice struct {
	gorm.Model
	OrganizationID uint `gorm:"not null"`
	Organization   Organization
	UserID         uint // user who triggered the invoice 
	User           User
	Amount         float64   `gorm:"not null"`
	Currency       string    `gorm:"not null;default:'USD'"`
	IssueDate      time.Time `gorm:"not null"`
	DueDate        time.Time `gorm:"not null"`
	Paid           bool      `gorm:"default:false"`
	Payments       []Payment
	Refunds        []Refund
}

type Payment struct {
	gorm.Model
	InvoiceID     uint `gorm:"not null"`
	Invoice       Invoice
	UserID        uint // user who made the payment
	User          User
	Amount        float64   `gorm:"not null"`
	Currency      string    `gorm:"not null;default:'USD'"`
	PaymentDate   time.Time `gorm:"not null"`
	TransactionID string    `gorm:"unique;not null"`
	PaymentMethod string
}

type Refund struct {
	gorm.Model
	InvoiceID     uint `gorm:"not null"`
	Invoice       Invoice
	PaymentID     uint 
	Payment       Payment
	UserID        uint 
	User          User
	Amount        float64   `gorm:"not null"`
	Currency      string    `gorm:"not null;default:'USD'"`
	RefundDate    time.Time `gorm:"not null"`
	TransactionID string    `gorm:"unique;not null"`
	Reason        string
}
