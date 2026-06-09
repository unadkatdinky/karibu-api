package utils

import (
	"regexp"
	"unicode"
)

type PasswordError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func ValidatePassword(password string) []PasswordError {
	var errors []PasswordError
	
	// Minimum length
	if len(password) < 8 {
		errors = append(errors, PasswordError{
			Field:   "password",
			Message: "Password must be at least 8 characters long",
		})
	}
	
	// Maximum length (prevent DOS)
	if len(password) > 128 {
		errors = append(errors, PasswordError{
			Field:   "password",
			Message: "Password must not exceed 128 characters",
		})
	}
	
	// Uppercase letter
	if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		errors = append(errors, PasswordError{
			Field:   "password",
			Message: "Password must contain at least one uppercase letter (A-Z)",
		})
	}
	
	// Lowercase letter
	if !regexp.MustCompile(`[a-z]`).MatchString(password) {
		errors = append(errors, PasswordError{
			Field:   "password",
			Message: "Password must contain at least one lowercase letter (a-z)",
		})
	}
	
	// Number
	if !regexp.MustCompile(`[0-9]`).MatchString(password) {
		errors = append(errors, PasswordError{
			Field:   "password",
			Message: "Password must contain at least one number (0-9)",
		})
	}
	
	// Special character
	if !regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(password) {
		errors = append(errors, PasswordError{
			Field:   "password",
			Message: "Password must contain at least one special character (!@#$%^&* etc)",
		})
	}
	
	// No common patterns
	commonPatterns := []string{
		"123456", "password", "qwerty", "abc123",
		"letmein", "welcome", "monkey", "dragon",
	}
	
	for _, pattern := range commonPatterns {
		if regexp.MustCompile(`(?i)` + pattern).MatchString(password) {
			errors = append(errors, PasswordError{
				Field:   "password",
				Message: "Password contains common patterns. Please choose something unique",
			})
			break
		}
	}
	
	return errors
}

func ValidateEmail(email string) []PasswordError {
	var errors []PasswordError
	
	if !regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`).MatchString(email) {
		errors = append(errors, PasswordError{
			Field:   "email",
			Message: "Invalid email format",
		})
	}
	
	if len(email) > 254 {
		errors = append(errors, PasswordError{
			Field:   "email",
			Message: "Email is too long",
		})
	}
	
	return errors
}

func ValidateFullName(name string) []PasswordError {
	var errors []PasswordError
	
	if len(name) < 2 {
		errors = append(errors, PasswordError{
			Field:   "fullName",
			Message: "Full name must be at least 2 characters",
		})
	}
	
	if len(name) > 100 {
		errors = append(errors, PasswordError{
			Field:   "fullName",
			Message: "Full name must not exceed 100 characters",
		})
	}
	
	// Check if contains only valid characters
	for _, char := range name {
		if !unicode.IsLetter(char) && !unicode.IsSpace(char) && char != '-' && char != '\'' {
			errors = append(errors, PasswordError{
				Field:   "fullName",
				Message: "Full name contains invalid characters",
			})
			break
		}
	}
	
	return errors
}
