// pkg/valueobjects/money_test.go
package valueobjects

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMoney(t *testing.T) {
	tests := []struct {
		name        string
		amount      decimal.Decimal
		currency    Currency
		shouldError bool
	}{
		{
			name:        "valid money",
			amount:      decimal.NewFromFloat(10.99),
			currency:    USD,
			shouldError: false,
		},
		{
			name:        "negative amount",
			amount:      decimal.NewFromFloat(-10.99),
			currency:    USD,
			shouldError: true,
		},
		{
			name:        "invalid currency",
			amount:      decimal.NewFromFloat(10.99),
			currency:    "XXX",
			shouldError: true,
		},
		{
			name:        "too many decimal places",
			amount:      decimal.NewFromFloat(10.999),
			currency:    USD,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money, err := NewMoney(tt.amount, tt.currency)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, money)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, money)
				assert.Equal(t, tt.amount, money.Amount())
				assert.Equal(t, tt.currency, money.Currency())
			}
		})
	}
}

func TestMoneyArithmetic(t *testing.T) {
	// Setup test money values
	tenUSD, err := NewMoney(decimal.NewFromFloat(10.00), USD)
	require.NoError(t, err)

	fiveUSD, err := NewMoney(decimal.NewFromFloat(5.00), USD)
	require.NoError(t, err)

	tenEUR, err := NewMoney(decimal.NewFromFloat(10.00), EUR)
	require.NoError(t, err)

	t.Run("addition - same currency", func(t *testing.T) {
		result, err := tenUSD.Add(*fiveUSD)
		assert.NoError(t, err)
		assert.Equal(t, decimal.NewFromFloat(15.00), result.Amount())
		assert.Equal(t, USD, result.Currency())
	})

	t.Run("addition - different currency", func(t *testing.T) {
		result, err := tenUSD.Add(*tenEUR)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("subtraction - same currency", func(t *testing.T) {
		result, err := tenUSD.Subtract(*fiveUSD)
		assert.NoError(t, err)
		assert.Equal(t, decimal.NewFromFloat(5.00), result.Amount())
	})

	t.Run("subtraction - would go negative", func(t *testing.T) {
		result, err := fiveUSD.Subtract(*tenUSD)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestMoneySplit(t *testing.T) {
	tests := []struct {
		name        string
		amount      decimal.Decimal
		parts       int
		shouldError bool
		expected    []decimal.Decimal
	}{
		{
			name:        "even split",
			amount:      decimal.NewFromFloat(10.00),
			parts:       2,
			shouldError: false,
			expected:    []decimal.Decimal{decimal.NewFromFloat(5.00), decimal.NewFromFloat(5.00)},
		},
		{
			name:        "uneven split",
			amount:      decimal.NewFromFloat(10.00),
			parts:       3,
			shouldError: false,
			expected:    []decimal.Decimal{decimal.NewFromFloat(3.34), decimal.NewFromFloat(3.33), decimal.NewFromFloat(3.33)},
		},
		{
			name:        "invalid parts",
			amount:      decimal.NewFromFloat(10.00),
			parts:       0,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money, err := NewMoney(tt.amount, USD)
			require.NoError(t, err)

			splits, err := money.Split(tt.parts)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, splits)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, splits, tt.parts)

			// Verify split amounts
			for i, split := range splits {
				expected := tt.expected[i].Round(2)
				actual := split.Amount().Round(2)
				assert.True(t, expected.Equal(actual), "unexpected split amount: expected %s, got %s", expected, actual)
				assert.Equal(t, USD, split.Currency())
			}

			// Verify sum equals original
			sum := decimal.Zero
			for _, split := range splits {
				sum = sum.Add(split.Amount())
			}
			assert.True(t, money.Amount().Equal(sum))
		})
	}
}

func TestMoneyFromString(t *testing.T) {
	tests := []struct {
		name        string
		amount      string
		currency    string
		shouldError bool
		expected    decimal.Decimal
	}{
		{
			name:        "valid amount",
			amount:      "10.99",
			currency:    "USD",
			shouldError: false,
			expected:    decimal.NewFromFloat(10.99),
		},
		{
			name:        "invalid amount",
			amount:      "abc",
			currency:    "USD",
			shouldError: true,
		},
		{
			name:        "invalid currency",
			amount:      "10.99",
			currency:    "XXX",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money, err := NewMoneyFromString(tt.amount, tt.currency)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, money)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, money)
				assert.True(t, tt.expected.Equal(money.Amount()))
				assert.Equal(t, Currency(tt.currency), money.Currency())
			}
		})
	}
}
