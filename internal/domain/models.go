// Package domain holds the core entities of the Gophermart loyalty system and
// the sentinel errors that the service layer uses to signal business outcomes.
package domain

import "time"

// OrderStatus is the processing state of an uploaded order as exposed to the user.
type OrderStatus string

// Order processing states.
const (
	OrderStatusNew        OrderStatus = "NEW"
	OrderStatusProcessing OrderStatus = "PROCESSING"
	OrderStatusInvalid    OrderStatus = "INVALID"
	OrderStatusProcessed  OrderStatus = "PROCESSED"
)

// User is a registered account.
type User struct {
	ID           int64
	Login        string
	PasswordHash string
	CreatedAt    time.Time
}

// Order is an order number submitted by a user for accrual calculation.
type Order struct {
	Number     string
	UserID     int64
	Status     OrderStatus
	Accrual    Money
	HasAccrual bool
	UploadedAt time.Time
}

// Withdrawal is a single debit of points against a hypothetical new order.
type Withdrawal struct {
	OrderNumber string
	UserID      int64
	Sum         Money
	ProcessedAt time.Time
}

// Balance is a user's current point balance and lifetime withdrawn total.
type Balance struct {
	Current   Money
	Withdrawn Money
}
