package boltmail

import (
	"fmt"
	"github.com/alienscience/imapsrv"
	"github.com/boltdb/bolt"
	"strconv"
	"strings"
)

type boltMailbox struct {
	store *BoltMailstore
	owner string

	path       []string
	flags      uint8
	currentUID int32

	children []*boltMailbox

	/*
		// Get the path of the mailbox
		Path() []string
		// Get the mailbox flags
		Flags() (uint8, error)
		// Get the uid validity value
		UidValidity() (int32, error)
		// Get the next available uid in the mailbox
		NextUid() (int32, error)
		// Get a list of all the uids in the mailbox
		AllUids() ([]int32, error)
		// Get the uid of the first unseen message
		FirstUnseen() (int32, error)
		// Get the total number of messages
		TotalMessages() (int32, error)
		// Get the total number of unread messages
		RecentMessages() (int32, error)
		// Fetch the message with the given UID
		Fetch(uid int32) (Message, error)
	*/
}

var (
	mail_bucket        = []byte("mail")
	firstUnseen_bucket = []byte("firstUnseen")
	total_bucket       = []byte("total")
	recent_bucket      = []byte("recent")
)

func (b *boltMailbox) Path() []string {
	return b.path
}

func (b *boltMailbox) Flags() (uint8, error) {
	return b.flags, nil
}

func (b *boltMailbox) UidValidity() (int32, error) {
	return -1, fmt.Errorf("UidValidity() not implemented")
}

func (b *boltMailbox) NextUid() (int32, error) {
	// TODO: make it safe for concurrent access
	return b.currentUID + 1, nil
	// TODO: should we update the value here, or at the Mailstore.NewMessage?
}

func (b *boltMailbox) AllUids() (uids []int32, err error) {
	err = b.store.connection.View(func(tx *bolt.Tx) error {
		mailboxBucket, err := b.getMailboxBucket(tx)
		if err != nil {
			return err
		}

		mailsBucket := mailboxBucket.Bucket(mail_bucket)
		if mailsBucket == nil {
			return fmt.Errorf("Bucket not found: %s", string(mail_bucket))
		}

		err = mailsBucket.ForEach(func(k, v []byte) error {
			uid, err := strconv.ParseInt(string(k), 10, 32)
			if err != nil {
				return err
			}
			uids = append(uids, int32(uid))
			return nil
		})
		return err
	})
	return
}

// FirstUnseen gets the uid of the first unseen message
// TODO: define "first": (longest ago, or most recent?)
func (b *boltMailbox) FirstUnseen() (uid int32, err error) {
	err = b.store.connection.View(func(tx *bolt.Tx) error {
		mailboxBucket, err := b.getMailboxBucket(tx)
		if err != nil {
			return err
		}

		uid_b := mailboxBucket.Get(firstUnseen_bucket)
		uid_64, err := strconv.ParseInt(string(uid_b), 10, 32)
		if err != nil {
			return err
		}

		uid = int32(uid_64)
		return nil
	})
	return
}

func (b *boltMailbox) TotalMessages() (total int32, err error) {
	err = b.store.connection.View(func(tx *bolt.Tx) error {
		mailboxBucket, err := b.getMailboxBucket(tx)
		if err != nil {
			return err
		}

		total_b := mailboxBucket.Get(total_bucket)
		totalRecent, err := strconv.ParseInt(string(total_b), 10, 32)
		if err != nil {
			return err
		}

		total = int32(totalRecent)
		return nil
	})
	return
}

func (b *boltMailbox) RecentMessages() (total int32, err error) {
	err = b.store.connection.View(func(tx *bolt.Tx) error {
		mailboxBucket, err := b.getMailboxBucket(tx)
		if err != nil {
			return err
		}

		total_b := mailboxBucket.Get(recent_bucket)
		totalRecent, err := strconv.ParseInt(string(total_b), 10, 32)
		if err != nil {
			return err
		}

		total = int32(totalRecent)
		return nil
	})
	return
}

func (b *boltMailbox) Fetch(uid int32) (imapsrv.Message, error) {
	msg := &basicMessage{}

	var binary []byte
	err := b.store.connection.View(func(tx *bolt.Tx) error {
		mailsBucket, err := b.getMailsBucket(tx)
		if err != nil {
			return err
		}

		binary = mailsBucket.Get([]byte(strconv.Itoa(int(uid))))
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(binary) == 0 {
		return nil, fmt.Errorf("mail %d not found", uid)
	}

	err = fromBytes(binary, msg)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func (b *boltMailbox) getMailboxBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bucket := tx.Bucket([]byte(b.owner))
	if bucket == nil {
		return nil, fmt.Errorf("user bucket not found: %s", b.owner)
	}

	pathString := strings.Join(b.path, "/")
	mailboxBucket := bucket.Bucket([]byte(pathString))
	if mailboxBucket == nil {
		return nil, fmt.Errorf("mailbox bucket not found: %s", pathString)
	}

	return mailboxBucket, nil
}

func (b *boltMailbox) getMailsBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	mb, err := b.getMailboxBucket(tx)
	if err != nil {
		return nil, err
	}

	buck := mb.Bucket(mail_bucket)
	if buck == nil {
		return nil, fmt.Errorf("could not find bucket: %s", string(mail_bucket))
	}

	return buck, nil
}
