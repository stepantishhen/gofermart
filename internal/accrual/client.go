// Package accrual talks to the external loyalty points calculation service and
// runs the background worker that keeps uploaded orders up to date.
package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/stepantishhen/gofermart/internal/domain"
)

// Status is the calculation status reported by the accrual service.
type Status string

// Accrual service statuses.
const (
	StatusRegistered Status = "REGISTERED"
	StatusInvalid    Status = "INVALID"
	StatusProcessing Status = "PROCESSING"
	StatusProcessed  Status = "PROCESSED"
)

// Result is the accrual service's answer for a single order.
type Result struct {
	Order      string
	Status     Status
	Accrual    domain.Money
	HasAccrual bool
}

// ErrOrderNotRegistered means the accrual service returned 204: the order is not
// known to it yet.
var ErrOrderNotRegistered = errors.New("order is not registered in the accrual service")

// TooManyRequestsError carries the Retry-After delay from a 429 response.
type TooManyRequestsError struct {
	RetryAfter time.Duration
}

// Error implements error.
func (e *TooManyRequestsError) Error() string {
	return fmt.Sprintf("accrual service rate limited, retry after %s", e.RetryAfter)
}

// Client is an HTTP client for the accrual service.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient builds a Client for the given base address.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

type apiResponse struct {
	Order   string   `json:"order"`
	Status  Status   `json:"status"`
	Accrual *float64 `json:"accrual"`
}

// Get fetches the accrual result for number. It returns ErrOrderNotRegistered on
// 204 and *TooManyRequestsError on 429.
func (c *Client) Get(ctx context.Context, number string) (Result, error) {
	url := c.baseURL + "/api/orders/" + number
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{}, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var body apiResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return Result{}, fmt.Errorf("decode accrual response: %w", err)
		}
		res := Result{Order: body.Order, Status: body.Status}
		if body.Accrual != nil {
			res.Accrual = domain.NewMoneyFromFloat(*body.Accrual)
			res.HasAccrual = true
		}
		return res, nil
	case http.StatusNoContent:
		return Result{}, ErrOrderNotRegistered
	case http.StatusTooManyRequests:
		return Result{}, &TooManyRequestsError{RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	default:
		return Result{}, fmt.Errorf("accrual service returned status %d", resp.StatusCode)
	}
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return time.Minute
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
		return time.Duration(secs) * time.Second
	}
	return time.Minute
}
