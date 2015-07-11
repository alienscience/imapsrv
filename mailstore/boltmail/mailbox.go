package boltmail

import (
	"fmt"
	"strconv"
	"strings"

	"bytes"

	"github.com/alienscience/imapsrv"
	"github.com/boltdb/bolt"
)

type boltMailbox struct {
	uid    int32
	uidSet bool
	store  *BoltMailstore
	owner  string

	path       []string
	flags      imapsrv.MailboxFlag
	subscribed bool
	metaSet    bool

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
	recent_key      = []byte("recent")
	uid_key         = []byte("uid")
	sub_key         = []byte("subscriptions")
	flags_key       = []byte("flags")
	counter_key     = []byte("increment-counter")
)

func (b *boltMailbox) Path() []string {
	return b.path
}

// moveBucket moves the inner bucket with key 'oldkey' to a new bucket with key 'newkey'
// must be used within an Update-transaction
func moveBucket(oldParent, newParent *bolt.Bucket, oldkey, newkey []byte) error {
	oldBuck := oldParent.Bucket(oldkey)
	newBuck, err := newParent.CreateBucket(newkey)
	if err != nil {
		return err
	}

	err = oldBuck.ForEach(func(k, v []byte) error {
		if v == nil {
			// Nested bucket
			return moveBucket(oldBuck, newBuck, k, k)
		} else {
			// Regular value
			return newBuck.Put(k, v)
		}
	})
	if err != nil {
		return err
	}

	return oldParent.DeleteBucket(oldkey)
}

func (b *boltMailbox) Rename(newPath []string) error {
	npath := strings.Join(newPath, "/")
	opath := strings.Join(b.path, "/")

	err := b.store.connection.Update(func(tx *bolt.Tx) error {
		ownerBuck := tx.Bucket(usersBucket).Bucket([]byte(b.owner))
		if ownerBuck == nil {
			return fmt.Errorf("user not found: %s", b.owner)
		}

		c := ownerBuck.Cursor()
		prefix := []byte(opath)

		for k, v := c.Seek(prefix); bytes.HasPrefix(k, prefix); k, v = c.Next() {
			if k == nil && v == nil {
				break
			}
			if v == nil {
				// It's a bucket
				newKey := bytes.Replace(k, prefix, []byte(npath), 1) // just the 1st entry
				err := moveBucket(ownerBuck, ownerBuck, k, newKey)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	return err
}

func (b *boltMailbox) Flags() (imapsrv.MailboxFlag, error) {
	if !b.metaSet {
		err := b.store.connection.View(func(tx *bolt.Tx) error {
			return b.refreshTransaction(tx)
		})
		if err != nil {
			return b.flags, nil
		}
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
			uidBuck := owner.Bucket(uid_key)
			if uidBuck == nil {
				if tx.Writable() {
					uidBuck, err = owner.CreateBucket(uid_key)
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
		mailbox, e := b.getMailboxBucketTx(tx)
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
		mailboxBucket, err := b.getMailboxBucketTx(tx)
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
		mailboxBucket, err := b.getMailboxBucketTx(tx)
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
		mailboxBucket, err := b.getMailboxBucketTx(tx)
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
		mailboxBucket, err := b.getMailboxBucketTx(tx)
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
	mailbox, err := b.getMailboxBucketTx(tx)
	if err != nil {
		return err
	}

	b.uid, err = b.nextUidTransaction(mailbox)
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

	err = mail.Put([]byte(strconv.Itoa(int(b.uid))), val)
	if err != nil {
		return err
	}

	// Update first-unseen
	id_b := mailbox.Get(firstUnseen_key)
	if len(id_b) == 0 {
		// Set uid
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

func (b *boltMailbox) deleteTransaction(tx *bolt.Tx) error {
	ownerBuck := tx.Bucket(usersBucket).Bucket([]byte(b.owner))
	if ownerBuck == nil {
		return fmt.Errorf("user %s does not exist", b.owner)
	}
	return ownerBuck.DeleteBucket([]byte(strings.Join(b.path, "/")))
}

func (b *boltMailbox) refreshTransaction(tx *bolt.Tx) error {
	box, err := b.getMailboxBucketTx(tx)
	if err != nil {
		return err
	}

	flagsB := box.Get(flags_key)
	if len(flagsB) > 0 {
		num, err := strconv.Atoi(string(flagsB))
		if err != nil {
			return err
		}
		b.flags = imapsrv.MailboxFlag(num)
	}
	subB := box.Get(sub_key)
	b.subscribed = len(subB) > 0
	return nil
}

func (b *boltMailbox) saveMetaTx(tx *bolt.Tx) error {
	box, err := b.getMailboxBucketTx(tx)
	if err != nil {
		return err
	}

	err = box.Put(flags_key, []byte(strconv.Itoa(int(b.flags))))
	if err != nil {
		return err
	}

	sub_val := []byte{}
	if b.subscribed {
		sub_val = []byte{1}
	}
	return box.Put(sub_key, sub_val)

}

func (b *boltMailbox) setAttributeTransaction(tx *bolt.Tx, attr imapsrv.MailboxFlag) error {
	if !b.metaSet {
		var err error
		b.flags, err = b.Flags()
		if err != nil {
			return err
		}
	}

	if b.flags&imapsrv.Noselect == 0 {
		b.flags ^= imapsrv.Noselect
		return b.saveMetaTx(tx)
	} else {
		/* https://tools.ietf.org/html/rfc3501#section-6.3.4 >>
		It is an error to attempt to
		delete a name that has inferior hierarchical names and also has
		the \Noselect mailbox name attribute (see the description of the
		LIST response for more details).
		*/
		return imapsrv.DeleteError{b.path}
	}
}

func (b *boltMailbox) deleteOrNoselectTransaction(tx *bolt.Tx) error {
	children, err := b.getChildren()
	if err != nil {
		return err
	}

	if len(children) > 0 {
		return b.setAttributeTransaction(tx, imapsrv.Noselect)
	} else {
		return b.deleteTransaction(tx)
	}
}

func (b *boltMailbox) createTransaction(tx *bolt.Tx) error {
	userBuck := tx.Bucket(usersBucket).Bucket([]byte(b.owner))
	if userBuck == nil {
		return fmt.Errorf("user bucket not found: %s", b.owner)
	}

	pathString := strings.Join(b.path, "/")
	_, err := userBuck.CreateBucket([]byte(pathString))
	return err
}

func (b *boltMailbox) getMailboxBucketTx(tx *bolt.Tx) (*bolt.Bucket, error) {
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
	mb, err := b.getMailboxBucketTx(tx)
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

		var prefix []byte
		if len(b.path) == 0 {
			prefix = []byte(strings.Join(b.path, "/"))
		} else {
			prefix = []byte(strings.Join(b.path, "/") + "/")
		}

		for k, _ := c.Seek(prefix); len(k) > 0 && bytes.HasPrefix(k, prefix) && !bytes.Equal(k, prefix); k, _ = c.Next() {
			if bytes.Count(k, []byte("/")) > bytes.Count(prefix, []byte("/")) {
				continue // it is nested deeper
			}
			boxes = append(boxes, &boltMailbox{
				owner: b.owner,
				path:  strings.Split(string(k), "/"),
				store: b.store,
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

func (b *boltMailbox) Checkpoint() {

}

func (b *boltMailbox) Subscribe() error {
	return b.store.connection.Update(func(tx *bolt.Tx) error {
		owner := tx.Bucket(usersBucket).Bucket([]byte(b.owner))
		if owner == nil {
			return fmt.Errorf("user not found: %s", owner)
		}
		subs, err := owner.CreateBucketIfNotExists(sub_key)
		if err != nil {
			return err
		}
		err = subs.Put([]byte(strings.Join(b.path, "/")), []byte{1})
		if err != nil {
			return err
		}

		b.subscribed = true
		return b.saveMetaTx(tx)
	})
}

func (b *boltMailbox) Unsubscribe() error {
	return b.store.connection.Update(func(tx *bolt.Tx) error {
		owner := tx.Bucket(usersBucket).Bucket([]byte(b.owner))
		if owner == nil {
			return fmt.Errorf("user not found: %s", owner)
		}
		subs, err := owner.CreateBucketIfNotExists(sub_key)
		if err != nil {
			return err
		}
		err = subs.Delete([]byte(strings.Join(b.path, "/")))
		if err != nil {
			return err
		}

		b.subscribed = false
		return b.saveMetaTx(tx)
	})
}

func (b *boltMailbox) Subscribed() (bool, error) {
	if !b.metaSet {
		err := b.store.connection.View(func(tx *bolt.Tx) error {
			return b.refreshTransaction(tx)
		})
		if err != nil {
			return false, err
		}
		b.metaSet = true
	}
	return b.subscribed, nil
}

func (b *boltMailbox) SubscribedDescendant() (bool, error) {
	sub := false
	err := b.store.connection.View(func(tx *bolt.Tx) error {
		owner := tx.Bucket(usersBucket).Bucket([]byte(b.owner))
		if owner == nil {
			return fmt.Errorf("user not found: %s", b.owner)
		}

		subs := owner.Bucket(sub_key)
		if subs == nil {
			return nil
		}

		c := subs.Cursor()

		search := []byte(strings.Join(b.path, "/") + "/")
		for k, _ := c.Seek(search); k != nil; k, _ = c.Next() {
			if bytes.HasPrefix(k, search) && !bytes.Equal(search, k) {
				sub = true
			}
		}

		return nil
	})
	return sub, err
}
