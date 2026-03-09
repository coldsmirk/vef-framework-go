package password

import (
	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptMinCost = 4
	bcryptMaxCost = 31
)

type bcryptEncoder struct {
	cost int
}

// BcryptOption configures bcryptEncoder.
type BcryptOption func(*bcryptEncoder)

// WithBcryptCost sets the bcrypt cost factor (4-31).
// Higher cost increases security but also computation time.
func WithBcryptCost(cost int) BcryptOption {
	return func(e *bcryptEncoder) {
		e.cost = cost
	}
}

// NewBcryptEncoder creates a new bcrypt-based password encoder.
// Default cost is bcrypt.DefaultCost (10).
func NewBcryptEncoder(opts ...BcryptOption) Encoder {
	encoder := &bcryptEncoder{
		cost: bcrypt.DefaultCost,
	}

	for _, opt := range opts {
		opt(encoder)
	}

	return encoder
}

func (e *bcryptEncoder) Encode(password string) (string, error) {
	if e.cost < bcryptMinCost || e.cost > bcryptMaxCost {
		return "", ErrInvalidCost
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), e.cost)
	if err != nil {
		return "", err
	}

	return string(hashedPassword), nil
}

func (*bcryptEncoder) Matches(password, encodedPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(encodedPassword), []byte(password))

	return err == nil
}

func (e *bcryptEncoder) UpgradeEncoding(encodedPassword string) bool {
	cost, err := bcrypt.Cost([]byte(encodedPassword))
	if err != nil {
		return false
	}

	return cost < e.cost
}
