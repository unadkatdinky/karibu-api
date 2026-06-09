package models

import (
	"time"
)

type Role string

const (
	RoleTraveler   Role = "Traveler"
	RoleLocalGuide Role = "Local Guide"
	RoleAdmin      Role = "Admin"
)

type User struct {
	ID           string    `json:"id"`
	FullName     string    `json:"fullName"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // The hyphen ensures this never accidentally leaks to the frontend
	Role         Role      `json:"role"`
	GoogleID     *string   `json:"googleId,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}