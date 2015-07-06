package boltmail

import (
	"io"
	"io/ioutil"
	"os"
	"time"

	"fmt"

	"github.com/alienscience/imapsrv"
	"github.com/boltdb/bolt"
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

func (b *BoltMailstore) Mailbox(owner string, path []string) (box imapsrv.Mailbox, err error) {
	err = b.connection.View(func(tx *bolt.Tx) error {
		boltBox := &boltMailbox{
			owner: owner,
			path:  path,
			store: b,
		}
		if e, _ := boltBox.Exists(); !e {
			return fmt.Errorf("mailbox not found: %v", path) // TODO: injection danger? / security
		}
		box = boltBox
		return nil
	})
	return
}

func (b *BoltMailstore) Mailboxes(owner string, path []string) (boxes []imapsrv.Mailbox, err error) {
	box := &boltMailbox{
		owner: owner,
		path:  path,
		store: b,
	}
	return box.getChildren()
}

func (b *BoltMailstore) NewMessage(rcpt string, input io.Reader) (imapsrv.Message, error) {
	msg := &basicMessage{}
	var err error

	msg.body, err = ioutil.ReadAll(input)
	if err != nil {
		return nil, err
	}

	msg.internalDate = time.Now()
	msg.size = uint32(len(msg.body))

	err = b.connection.Update(func(tx *bolt.Tx) error {
		box := &boltMailbox{
			owner: rcpt,
			path:  []string{"INBOX"},
			store: b,
		}
		return box.storeTransaction(msg, tx)
	})
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func (b *BoltMailstore) NewUser(email string) error {
	err := b.connection.Update(func(tx *bolt.Tx) error {
		buck := tx.Bucket(usersBucket)
		_, err := buck.CreateBucketIfNotExists([]byte(email))
		return err
	})
	return err
}

func (b *BoltMailstore) Addresses() ([]string, error) {
	var messages []string

	err := b.connection.View(func(tx *bolt.Tx) error {
		buck := tx.Bucket(usersBucket)
		return buck.ForEach(func(k, _ []byte) error {
			messages = append(messages, string(k))
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return messages, nil
}
