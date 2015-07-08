package boltmail

import (
	"fmt"
	"strconv"
	"strings"

	"bytes"

	"log"

	"github.com/alienscience/imapsrv"
	"github.com/boltdb/bolt"
)

type boltMailbox struct {
	uid    int32
	uidSet bool
	store  *BoltMailstore
	owner  string

	path     []string
	flags    uint8
	flagsSet bool

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
	mail_bucket     = []byte("mail")
	firstUnseen_key = []byte("firstUnseen")
	total_bucket    = []byte("total")
	recent_key      = []byte("recent")
	uid_bucket      = []byte("uid")

	uid_key     = []byte("uid")
	counter_key = []byte("increment-counter")
)

func (b *boltMailbox) Path() []string {
	return b.path
}

func (b *boltMailbox) Flags() (uint8, error) {
	if !b.flagsSet {
		// TODO: fetch flags
	}
	return b.flags, nil
}

func (b *boltMailbox) UidValidity() (int32, error) {
	if !b.uidSet {
		// TODO: fetch uid
		err := b.store.connection.View(func(tx *bolt.Tx) error {
			owner := tx.Bucket(usersBucket).Bucket([]byte(b.owner))
			if owner == nil {
				return fmt.Errorf("user not found: %s", owner)
			}

			// Get the UID bucket
			var err error
			uidBuck := owner.Bucket(uid_bucket)
			if uidBuck == nil {
				if tx.Writable() {
					uidBuck, err = owner.CreateBucket(uid_bucket)
					if err != nil {
						return err
					}
					uidBuck.Put([]byte(strings.Join(b.path, "/")), []byte(strconv.Itoa(0)))
					uidBuck.Put(counter_key, []byte(strconv.Itoa(1)))
				}
				b.uid = 0
				return nil
			}
			// It existed, so get the value
			val := uidBuck.Get([]byte(strings.Join(b.path, "/")))
			if len(val) == 0 {
				// Did not have an entry? Set it if we can
				if tx.Writable() {
					prevValue, err := getInt(counter_key, uidBuck)
					if err != nil {
						return err
					}
					uidBuck.Put([]byte(strings.Join(b.path, "/")), []byte(strconv.Itoa(prevValue)))
					uidBuck.Put(counter_key, []byte(strconv.Itoa(prevValue+1)))
				}
				b.uid = 0
				return nil
			}
			// Parse it
			uid_int, err := strconv.Atoi(string(val))
			if err != nil {
				return err
			}
			b.uid = int32(uid_int)

			return nil
		})
		if err != nil {
			return -1, err
		}
	}
	return b.uid, nil
}

func (b *boltMailbox) NextUid() (uid int32, err error) {
	err = b.store.connection.Update(func(tx *bolt.Tx) error {
		mailbox, e := b.getMailboxBucket(tx)
		if e != nil {
			return e
		}
		uid, e = b.nextUidTransaction(mailbox)
		return e
	})
	return
}

// nextUidTransaction increments the uid by one, in a transaction in which `mailbox` is valid
func (b *boltMailbox) nextUidTransaction(mailbox *bolt.Bucket) (uid int32, err error) {
	val := mailbox.Get(uid_key)
	uid_int, e := strconv.Atoi(string(val))
	if e != nil {
		uid_int = 0
	}
	uid_int++
	uid = int32(uid_int)
	err = mailbox.Put(uid_key, []byte(strconv.Itoa(uid_int)))
	return
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

		uid_b := mailboxBucket.Get(firstUnseen_key)
		if len(uid_b) == 0 {
			uid = -1
			return nil
		}
		// TODO: what to return if no messages are present? (empty)
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
		mail := mailboxBucket.Bucket(mail_bucket)
		if mail == nil {
			return nil
		}
		total = int32(mail.Stats().KeyN)
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

		total_b := mailboxBucket.Get(recent_key)
		if len(total_b) == 0 {
			total = 0
			return nil
		}

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

	err = msg.GobDecode(binary)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func (b *boltMailbox) storeTransaction(msg *basicMessage, tx *bolt.Tx) error {
	mailbox, err := b.getMailboxBucket(tx)
	if err != nil {
		return err
	}

	uid, err := b.nextUidTransaction(mailbox)
	if err != nil {
		return err
	}

	mail, err := mailbox.CreateBucketIfNotExists(mail_bucket)
	if err != nil {
		return fmt.Errorf("Could not create /INBOX mail bucket: %s", err)
	}

	val, err := msg.GobEncode()
	if err != nil {
		return err
	}

	err = mail.Put([]byte(strconv.Itoa(int(uid))), val)
	if err != nil {
		return err
	}

	// Update first-unseen
	id_b := mailbox.Get(firstUnseen_key)
	if len(id_b) == 0 {
		// Set uid
		log.Println("Setting firstUnseen_key")
		// TODO: We would probably need to add +1 to make sure this != 0, but then we might
		// go out of range.... howerver, the specs require
		//	; Non-zero unsigned 32-bit integer
		//  ; (0 < n < 4,294,967,296)
		err = mailbox.Put(firstUnseen_key, []byte(strconv.Itoa(mail.Stats().KeyN)))
		if err != nil {
			return err
		}
	}

	// Update recent number
	recent_b := mailbox.Get(recent_key)
	recent := 0
	if len(recent_b) > 0 {
		recent, err = strconv.Atoi(string(recent_b))
		if err != nil {
			return err
		}
	}
	mailbox.Put(recent_key, []byte(strconv.Itoa(recent+1)))

	return nil
}

func (b *boltMailbox) getMailboxBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	users := tx.Bucket(usersBucket)
	bucket := users.Bucket([]byte(b.owner))
	if bucket == nil {
		return nil, fmt.Errorf("user bucket not found: %s", b.owner)
	}

	var mailboxBucket *bolt.Bucket
	var err error

	pathString := strings.Join(b.path, "/")
	if tx.Writable() {
		mailboxBucket, err = bucket.CreateBucketIfNotExists([]byte(pathString))
		if err != nil {
			return nil, fmt.Errorf("mailbox bucket not found and could not be created: %s", err)
		}
	} else {
		mailboxBucket = bucket.Bucket([]byte(pathString))
		if mailboxBucket == nil {
			return nil, fmt.Errorf("mailbox not found: %s", pathString)
		}
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

func (b *boltMailbox) getChildren() (boxes []imapsrv.Mailbox, err error) {
	err = b.store.connection.View(func(tx *bolt.Tx) error {
		users := tx.Bucket(usersBucket) // TODO: specific owner!
		owner := users.Bucket([]byte(b.owner))
		if owner == nil {
			return fmt.Errorf("user does not exist: %s", b.owner)
		}
		c := owner.Cursor()
		if c == nil {
			return fmt.Errorf("could not create cursor")
		}

		prefix := []byte(strings.Join(b.path, "/"))
		fmt.Println("Prefix:", string(prefix))
		for k, _ := c.Seek(prefix); len(k) > 0 && bytes.HasPrefix(k, prefix) && !bytes.Equal(k, prefix); k, _ = c.Next() {
			boxes = append(boxes, &boltMailbox{
				owner: b.owner,
				path:  strings.Split(string(k), "/"),
			})
		}
		return nil
	})
	return
}

func (b *boltMailbox) Exists() (exists bool, err error) {
	err = b.store.connection.View(func(tx *bolt.Tx) error {
		users := tx.Bucket(usersBucket)
		owner := users.Bucket([]byte(b.owner))
		if owner == nil {
			return fmt.Errorf("user not found: %s", owner) // TODO: potential injection threath?
		}
		exists = owner.Bucket([]byte(strings.Join(b.path, "/"))) != nil
		return nil
	})
	return
}
