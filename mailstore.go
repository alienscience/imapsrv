package imapsrv

import (
	"log"
)

// Mailbox represents an IMAP mailbox
type Mailbox struct {
	Name  string   // The name of the mailbox
	Path  []string // Full mailbox path
	Id    int64    // The id of the mailbox
	Flags uint8    // Mailbox flags
}

// Mailbox flags
const (
	// Noinferiors indicates it is not possible for any child levels of hierarchy to exist
	// under this name; no child levels exist now and none can be
	// created in the future.
	Noinferiors = 1 << iota

	// Noselect indicates it is not possible to use this name as a selectable mailbox.
	Noselect

	// Marked indicates that the mailbox has been marked "interesting" by the server;
	// the mailbox probably contains messages that have been added since
	// the last time the mailbox was selected
	Marked

	// Unmarked indicates the mailbox does not contain any additional messages since the
	// last time the mailbox was selected.
	Unmarked
)

var mailboxFlags = map[uint8]string{
	Noinferiors: "Noinferiors",
	Noselect:    "Noselect",
	Marked:      "Marked",
	Unmarked:    "Unmarked",
}

// Mailstore is a service responsible for I/O with the actual e-mails
type Mailstore interface {
	// GetMailbox gets IMAP mailbox information
	// Returns nil if the mailbox does not exist
	GetMailbox(path []string) (*Mailbox, error)
	// GetMailboxes gets a list of mailboxes at the given path
	GetMailboxes(path []string) ([]*Mailbox, error)
	// FirstUnseen gets the sequence number of the first unseen message in an IMAP mailbox
	FirstUnseen(mbox int64) (int64, error)
	// TotalMessages gets the total number of messages in an IMAP mailbox
	TotalMessages(mbox int64) (int64, error)
	// RecentMessages gets the total number of unread messages in an IMAP mailbox
	RecentMessages(mbox int64) (int64, error)
	// NextUid gets the next available uid in an IMAP mailbox
	NextUid(mbox int64) (int64, error)
}

// DummyMailstore is used for demonstrating the IMAP server
type dummyMailstore struct {
}

// GetMailbox gets mailbox information
func (m *dummyMailstore) GetMailbox(path []string) (*Mailbox, error) {
	return &Mailbox{
		Name: "inbox",
		Path: []string{"inbox"},
		Id:   1,
	}, nil
}

// GetMailboxes gets a list of mailboxes at the given path
func (m *dummyMailstore) GetMailboxes(path []string) ([]*Mailbox, error) {
	log.Printf("GetMailboxes %v", path)

	if len(path) == 0 {
		// Root
		return []*Mailbox{
			{
				Name: "inbox",
				Path: []string{"inbox"},
				Id:   1,
			},
			{
				Name: "spam",
				Path: []string{"spam"},
				Id:   2,
			},
		}, nil
	} else if len(path) == 1 && path[0] == "inbox" {
		return []*Mailbox{
			{
				Name: "starred",
				Path: []string{"inbox", "stared"},
				Id:   3,
			},
		}, nil
	} else {
		return []*Mailbox{}, nil
	}
}

// FirstUnseen gets the sequence number of the first unseen message in an IMAP mailbox
func (m *dummyMailstore) FirstUnseen(mbox int64) (int64, error) {
	return 4, nil
}

// TotalMessages gets the total number of messages in an IMAP mailbox
func (m *dummyMailstore) TotalMessages(mbox int64) (int64, error) {
	return 8, nil
}

// RecentMessages gets the total number of unread messages in an IMAP mailbox
func (m *dummyMailstore) RecentMessages(mbox int64) (int64, error) {
	return 4, nil
}

// DummyMailstore gets the next available uid in an IMAP mailbox
func (m *dummyMailstore) NextUid(mbox int64) (int64, error) {
	return 9, nil
}
