package hash

import "golang.org/x/crypto/bcrypt"

// PasswordHasher defines the contract for hashing and verifying passwords.
type PasswordHasher interface {
	HashPassword(password string) (string, error)
	ComparePassword(hashedPassword, password string) error
}

type bcryptHasher struct {
	cost int
}

// NewBcryptHasher creates a new instance of PasswordHasher using bcrypt.
func NewBcryptHasher(cost int) PasswordHasher {
	if cost <= 0 {
		cost = bcrypt.DefaultCost
	}
	return &bcryptHasher{cost: cost}
}

func (h *bcryptHasher) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (h *bcryptHasher) ComparePassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
