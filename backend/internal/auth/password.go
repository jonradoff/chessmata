package auth

import (
	"errors"
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost        = 12
	minPasswordLength = 10
)

var (
	ErrPasswordTooShort = errors.New("password must be at least 10 characters")
	ErrPasswordTooWeak  = errors.New("password must contain at least one uppercase letter, one lowercase letter, one number, and one special character")
	ErrPasswordCommon   = errors.New("this password is too common, please choose a more unique password")
)

// commonPasswords is a list of frequently breached passwords to reject.
var commonPasswords = map[string]bool{
	"password":    true, "password1":   true, "password123": true,
	"123456":      true, "1234567":     true, "12345678":    true,
	"123456789":   true, "1234567890":  true, "qwerty":      true,
	"qwerty123":   true, "abc123":      true, "monkey":      true,
	"dragon":      true, "letmein":     true, "trustno1":    true,
	"baseball":    true, "iloveyou":    true, "master":      true,
	"sunshine":    true, "ashley":      true, "michael":     true,
	"shadow":      true, "123123":      true, "654321":      true,
	"superman":    true, "qazwsx":      true, "football":    true,
	"password12":  true, "starwars":    true, "admin":       true,
	"welcome":     true, "hello":       true, "charlie":     true,
	"donald":      true, "login":       true, "princess":    true,
	"master123":   true, "welcome1":    true, "p@ssw0rd":    true,
	"passw0rd":    true, "pa$$word":    true, "changeme":    true,
	"chess":       true, "chessmata":   true, "chess123":    true,
}

type PasswordService struct {
	cost int
}

func NewPasswordService() *PasswordService {
	return &PasswordService{
		cost: bcryptCost,
	}
}

// HashPassword hashes a plain text password using bcrypt
func (s *PasswordService) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), s.cost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ComparePassword compares a plain text password with a hash
func (s *PasswordService) ComparePassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// ValidatePasswordStrength checks if a password meets minimum requirements
func (s *PasswordService) ValidatePasswordStrength(password string) error {
	if len(password) < minPasswordLength {
		return ErrPasswordTooShort
	}

	// Check against common passwords
	if commonPasswords[strings.ToLower(password)] {
		return ErrPasswordCommon
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if !hasUpper || !hasLower || !hasNumber || !hasSpecial {
		return ErrPasswordTooWeak
	}

	return nil
}
