package boltmail

import (
	"github.com/alienscience/imapsrv"
	"github.com/boltdb/bolt"
	"io"
	"io/ioutil"
	"os"
	"time"
)

type BoltMailstore struct {
	connection *bolt.DB
	/*
		// Get IMAP mailbox information
		// Returns nil if the mailbox does not exist
		Mailbox(path []string) (Mailbox, error)
		// Get a list of mailboxes at the given path
		Mailboxes(path []string) ([]Mailbox, error)
	*/
}

var (
	usersBucket = []byte("users")
)

// NewBoltAuthStore creates a new auth store using BoltDB, at the specified file location
func NewBoltMailstore(filename string) (*BoltMailstore, error) {
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

	store := &BoltMailstore{c}

	return store, nil
}

func (b *BoltMailstore) Mailbox(path []string) (imapsrv.Mailbox, error) {
	return nil, nil // TODO: Implement
}

func (b *BoltMailstore) Mailboxes(path []string) ([]imapsrv.Mailbox, error) {
	return nil, nil // TODO: Implement
}

func (b *BoltMailstore) NewMessage(input io.Reader) (imapsrv.Message, error) {
	msg := &basicMessage{}
	var err error

	msg.body, err = ioutil.ReadAll(input)
	if err != nil {
		return nil, err
	}

	msg.internalDate = time.Now()
	msg.size = uint32(len(msg.body))

	return msg, nil
}
