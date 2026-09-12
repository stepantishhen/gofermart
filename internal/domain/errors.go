package domain

import "errors"

// Business-outcome errors returned by the service layer and mapped to HTTP
// status codes by the handlers.
var (
	ErrLoginTaken          = errors.New("login already taken")
	ErrInvalidCredentials  = errors.New("invalid login/password pair")
	ErrOrderOwnedByUser    = errors.New("order already uploaded by this user")
	ErrOrderOwnedByAnother = errors.New("order already uploaded by another user")
	ErrInvalidOrderNumber  = errors.New("order number failed the Luhn check")
	ErrInsufficientBalance = errors.New("not enough points on the balance")
	ErrUserNotFound        = errors.New("user not found")
)
