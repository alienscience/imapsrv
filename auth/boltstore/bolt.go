// Package boltstore holds an implementation of github.com/alienscience/imapsrv/auth - AuthStore, using
// github.com/boltdb/bolt - DB.
package boltstore

import (
	"fmt"
	"github.com/alienscience/imapsrv/auth"
	"github.com/boltdb/bolt"
	"os"
)

type BoltAuthStore struct {
	connection *bolt.DB
}

var (
	usersBucket = []byte("users")
)

// NewBoltAuthStore creates a new auth store using BoltDB, at the specified file location
func NewBoltAuthStore(filename string) (*BoltAuthStore, error) {
	// Open database
	c, err := bolt.Open(filename, os.FileMode(600), nil)
	if err != nil {
		return nil, err
	}

	// Make sure the Buckets exist
	err = c.Update(func(tx *bolt.Tx) (e error) {
		_, e = tx.CreateBucketIfNotExists(usersBucket)
		if e != nil {
			return
		}

		return
	})
	if err != nil {
		return nil, err
	}

	store := &BoltAuthStore{c}

	return store, nil
}

// Authenticate attempts to authenticate the given credentials
func (b *BoltAuthStore) Authenticate(username, plainPassword string) (success bool, err error) {
	// TODO: do we want this check here, or in a separate "IsAvailable" method in the interface?
	if b.connection == nil {
		return false, auth.ErrNotConnected
	}

	var hashedPassword []byte

	err = b.connection.View(func(tx *bolt.Tx) error {
		buck := tx.Bucket(usersBucket)
		hashedPassword = buck.Get([]byte(username))
		return nil
	})
	if err != nil {
		return false, err
	}
	if len(hashedPassword) == 0 {
		return false, fmt.Errorf("user %s not found", username)
	}

	return auth.CheckPassword([]byte(plainPassword), hashedPassword), nil
}

// CreateUser creates a user with the given username
func (b *BoltAuthStore) CreateUser(username, plainPassword string) error {
	if b.connection == nil {
		return auth.ErrNotConnected
	}

	hashedPassword, err := auth.HashPassword([]byte(plainPassword))
	if err != nil {
		return err
	}

	err = b.connection.Update(func(tx *bolt.Tx) error {
		buck := tx.Bucket(usersBucket)
		return buck.Put([]byte(username), hashedPassword)
	})
	return err
}

// ResetPassword resets the password for the given username
func (b *BoltAuthStore) ResetPassword(username, plainPassword string) error {
	if b.connection == nil {
		return auth.ErrNotConnected
	}
	// TODO: implement
	return nil
}

// ListUsers lists all information about the users
// TODO: this could be very neat for the sysadmin, but probably a lot of metadata
// 		 about users is desired, and not just the usernames.
func (b *BoltAuthStore) ListUsers() (usernames []string, err error) {
	if b.connection == nil {
		return []string{}, auth.ErrNotConnected
	}
	// TODO: implement
	return []string{}, nil
}

// DeleteUser removes the username from the database entirely
func (b *BoltAuthStore) DeleteUser(username string) error {
	if b.connection == nil {
		return auth.ErrNotConnected
	}
	// TODO: implement
	return nil
}
