// pkg/valueobjects/money.go
package valueobjects

import (
	"fmt"
	"strings"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/shopspring/decimal"
)

// Currency represents a valid ISO 4217 currency code
type Currency string

// Supported currencies
const (
	USD Currency = "USD"
	EUR Currency = "EUR"
	GBP Currency = "GBP"
	// Add more as needed
)

// validCurrencies maintains a set of supported currencies
var validCurrencies = map[Currency]bool{
	USD: true,
	EUR: true,
	GBP: true,
}

// Money represents a monetary value with a specific currency
type Money struct {
	amount   decimal.Decimal
	currency Currency
}

// NewMoney creates a new Money instance with validation
func NewMoney(amount decimal.Decimal, currency Currency) (*Money, error) {
	if !isValidCurrency(currency) {
		return nil, errors.ValidationFailed(
			"invalid currency",
			fmt.Sprintf("currency %s is not supported", currency),
		)
	}

	if amount.LessThan(decimal.Zero) {
		return nil, errors.ValidationFailed(
			"invalid amount",
			"amount cannot be negative",
		)
	}

	// Ensure amount has max 2 decimal places
	if amount.Exponent() < -2 {
		return nil, errors.ValidationFailed(
			"invalid amount",
			"amount cannot have more than 2 decimal places",
		)
	}

	return &Money{
		amount:   amount,
		currency: currency,
	}, nil
}

// NewMoneyFromString creates a Money instance from string representations
func NewMoneyFromString(amount string, currency string) (*Money, error) {
	// Parse decimal amount
	decimalAmount, err := decimal.NewFromString(amount)
	if err != nil {
		return nil, errors.ValidationFailed(
			"invalid amount format",
			err.Error(),
		)
	}

	// Validate and convert currency
	curr := Currency(strings.ToUpper(currency))
	return NewMoney(decimalAmount, curr)
}

// Amount returns the decimal amount
func (m Money) Amount() decimal.Decimal {
	return m.amount
}

// Currency returns the currency code
func (m Money) Currency() Currency {
	return m.currency
}

// Add adds two monetary values of the same currency
func (m Money) Add(other Money) (*Money, error) {
	if m.currency != other.currency {
		return nil, errors.ValidationFailed(
			ErrCurrencyMismatch,
			fmt.Sprintf("cannot add %s to %s", other.currency, m.currency),
		)
	}

	return &Money{
		amount:   m.amount.Add(other.amount),
		currency: m.currency,
	}, nil
}

// Subtract subtracts two monetary values of the same currency
func (m Money) Subtract(other Money) (*Money, error) {
	if m.currency != other.currency {
		return nil, errors.ValidationFailed(
			"currency mismatch",
			fmt.Sprintf("cannot subtract %s from %s", other.currency, m.currency),
		)
	}

	result := m.amount.Sub(other.amount)
	if result.LessThan(decimal.Zero) {
		return nil, errors.ValidationFailed(
			"invalid operation",
			"subtraction would result in negative amount",
		)
	}

	return &Money{
		amount:   result,
		currency: m.currency,
	}, nil
}

// Split divides money into n equal parts
func (m Money) Split(n int) ([]*Money, error) {
	if n <= 0 {
		return nil, errors.ValidationFailed(
			"invalid split",
			"number of parts must be positive",
		)
	}

	// Calculate the total amount in cents to avoid floating-point issues
	totalAmount := m.amount.Mul(decimal.NewFromInt(100))                       // Convert to cents
	baseAmount := totalAmount.Div(decimal.NewFromInt(int64(n))).Floor()        // Base amount in cents
	remainder := totalAmount.Sub(baseAmount.Mul(decimal.NewFromInt(int64(n)))) // Remaining cents

	result := make([]*Money, n)

	for i := 0; i < n; i++ {
		partAmount := baseAmount
		if remainder.GreaterThan(decimal.Zero) {
			partAmount = partAmount.Add(decimal.NewFromInt(1)) // Distribute extra cents
			remainder = remainder.Sub(decimal.NewFromInt(1))
		}

		// Convert back to dollars and round to 2 decimal places
		result[i] = &Money{
			amount:   partAmount.Div(decimal.NewFromInt(100)).Round(2),
			currency: m.currency,
		}
	}

	return result, nil
}

// IsZero checks if the amount is zero
func (m Money) IsZero() bool {
	return m.amount.IsZero()
}

// Equals checks if two monetary values are equal
func (m Money) Equals(other Money) bool {
	return m.currency == other.currency && m.amount.Equal(other.amount)
}

// String returns a string representation of the money value
func (m Money) String() string {
	return fmt.Sprintf("%s %s", m.amount.String(), m.currency)
}

// IsGreaterThan checks if this amount is greater than another
func (m Money) IsGreaterThan(other Money) (bool, error) {
	if m.currency != other.currency {
		return false, errors.ValidationFailed(
			"currency mismatch",
			fmt.Sprintf("cannot compare %s with %s", m.currency, other.currency),
		)
	}
	return m.amount.GreaterThan(other.amount), nil
}

// private helpers
func isValidCurrency(currency Currency) bool {
	return validCurrencies[currency]
}

const (
	ErrInvalidAmount    = "INVALID_AMOUNT"
	ErrInvalidCurrency  = "INVALID_CURRENCY"
	ErrCurrencyMismatch = "CURRENCY_MISMATCH"
)

type ValidationResult struct {
	Valid   bool
	Code    string
	Message string
}

func (m Money) Validate() ValidationResult {
	if m.amount.LessThan(decimal.Zero) {
		return ValidationResult{
			Valid:   false,
			Code:    ErrInvalidAmount,
			Message: "amount cannot be negative",
		}
	}

	if !isValidCurrency(m.currency) {
		return ValidationResult{
			Valid:   false,
			Code:    ErrInvalidCurrency,
			Message: fmt.Sprintf("currency %s is not supported", m.currency),
		}
	}

	return ValidationResult{Valid: true}
}

func NewMoneyFromInt(amount int64, currency Currency) (*Money, error) {
	return NewMoney(decimal.NewFromInt(amount), currency)
}

func (m Money) IsPositive() bool {
	return m.amount.GreaterThan(decimal.Zero)
}

func (m Money) Compare(other Money) (int, error) {
	if m.currency != other.currency {
		return 0, errors.ValidationFailed(
			ErrCurrencyMismatch,
			fmt.Sprintf("cannot compare %s with %s", m.currency, other.currency),
		)
	}
	return m.amount.Cmp(other.amount), nil
}
