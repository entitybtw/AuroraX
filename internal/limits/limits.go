package limits

import (
	"context"
	"errors"
	"time"

	"aurora/configuration"
	"aurora/internal/storage"
)

var ErrNotFound = errors.New("budget not found")

const (
	StatusWarning = "warning"
	StatusBlocked = "blocked"
)

type Budget struct {
	UserPath      string
	PeriodSeconds int
	Amount        float64
	WebhookURL    string
}

type CheckResult struct {
	Budget    Budget
	Status    string
	Spent     float64
	Remaining float64
	PeriodEnd time.Time
}

func (r CheckResult) UsageRatio() float64 {
	if r.Budget.Amount <= 0 {
		return 0
	}
	return r.Spent / r.Budget.Amount
}

type Reservation struct {
	ID        string
	UserPath  string
	Amount    float64
	CreatedAt time.Time
}

type ExceededError struct{ Result CheckResult }

func (e *ExceededError) Error() string { return "budget exceeded" }

type Service struct{}

func (s *Service) Check(context.Context, string, time.Time) error { return nil }
func (s *Service) CheckWithWarnings(context.Context, string, time.Time) ([]CheckResult, error) {
	return nil, nil
}
func (s *Service) Reserve(_ context.Context, reservationID string, userPath string, amount float64, now time.Time) (*Reservation, error) {
	return &Reservation{ID: reservationID, UserPath: userPath, Amount: amount, CreatedAt: now}, nil
}
func (s *Service) ReconcileReservation(string, float64) {}
func (s *Service) ReleaseReservation(string)            {}

type Result struct {
	Service *Service
	Storage storage.Storage
}

func New(context.Context, *config.Config) (*Result, error) { return &Result{}, nil }
func NewWithSharedStorage(context.Context, *config.Config, storage.Storage) (*Result, error) {
	return &Result{}, nil
}
func (r *Result) Close() error { return nil }

func PeriodLabel(seconds int) string { return "period-" + string(rune(seconds)) }
