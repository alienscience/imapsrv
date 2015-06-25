package auth

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrNotConnected = fmt.Errorf("database not connected")
)

// AuthStore contacts the backend to query about the users
type AuthStore interface {
	// Authenticate attempts to authenticate the given credentials
	Authenticate(username, plainPassword string) (success bool, err error)

	// CreateUser creates a user with the given username
	CreateUser(username, plainPassword string) error

	// ResetPassword resets the password for the given username
	ResetPassword(username, plainPassword string) error

	// ListUsers lists all information about the users
	// TODO: this could be very neat for the sysadmin, but probably a lot of metadata
	// 		 about users is desired, and not just the usernames.
	ListUsers() (usernames []string, err error)

	// DeleteUser removes the username from the database entirely
	DeleteUser(username string) error
}

// CheckPassword checks if the hash was the result of hashing this specific plainPassword
func CheckPassword(plainPassword, hash []byte) bool {
	return bcrypt.CompareHashAndPassword(hash, plainPassword) == nil
}

// HashPassword hashes the plainPassword using the bcrypt.DefaultCost
func HashPassword(plainPassword []byte) ([]byte, error) {
	return bcrypt.GenerateFromPassword(plainPassword, bcrypt.DefaultCost)
}
