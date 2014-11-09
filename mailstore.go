package imapsrv

import (
	"log"
)

// An IMAP mailbox
type Mailbox struct {
	Name  string   // The name of the mailbox
	Path  []string // Full mailbox path
	Id    int64    // The id of the mailbox
	Flags uint8    // Mailbox flags
}

// Mailbox flags
const (
	// It is not possible for any child levels of hierarchy to exist
	// under this name; no child levels exist now and none can be
	// created in the future.
	Noinferiors = 1 << iota
	// It is not possible to use this name as a selectable mailbox.
	Noselect
	// The mailbox has been marked "interesting" by the server; the
	// mailbox probably contains messages that have been added since
	// the last time the mailbox was selected
	Marked
	// The mailbox does not contain any additional messages since the
	// last time the mailbox was selected.
	Unmarked
)

var mailboxFlags = map[uint8]string{
	Noinferiors: "Noinferiors",
	Noselect:    "Noselect",
	Marked:      "Marked",
	Unmarked:    "Unmarked",
}

// A service that is needed to read mail messages
type Mailstore interface {
	// Get IMAP mailbox information
	// Returns nil if the mailbox does not exist
	GetMailbox(path []string) (*Mailbox, error)
	// Get a list of mailboxes at the given path
	GetMailboxes(path []string) ([]*Mailbox, error)
	// Get the sequence number of the first unseen message
	FirstUnseen(mbox int64) (int64, error)
	// Get the total number of messages in an IMAP mailbox
	TotalMessages(mbox int64) (int64, error)
	// Get the total number of unread messages in an IMAP mailbox
	RecentMessages(mbox int64) (int64, error)
	// Get the next available uid in an IMAP mailbox
	NextUid(mbox int64) (int64, error)
}

// A dummy mailstore used for demonstrating the IMAP server
type DummyMailstore struct {
}

// Get mailbox information
func (m *DummyMailstore) GetMailbox(path []string) (*Mailbox, error) {
	return &Mailbox{
		Name: "inbox",
		Path: []string{"inbox"},
		Id:   1,
	}, nil
}

// Get a list of mailboxes at the given path
func (m *DummyMailstore) GetMailboxes(path []string) ([]*Mailbox, error) {
	log.Printf("GetMailboxes %v", path)

	if len(path) == 0 {
		// Root
		return []*Mailbox{
			&Mailbox{
				Name: "inbox",
				Path: []string{"inbox"},
				Id:   1,
			},
			&Mailbox{
				Name: "spam",
				Path: []string{"spam"},
				Id:   2,
			},
		}, nil
	} else if len(path) == 1 && path[0] == "inbox" {
		return []*Mailbox{
			&Mailbox{
				Name: "starred",
				Path: []string{"inbox", "stared"},
				Id:   3,
			},
		}, nil
	} else {
		return []*Mailbox{}, nil
	}
}

// Get the sequence number of the first unseen message
func (m *DummyMailstore) FirstUnseen(mbox int64) (int64, error) {
	return 4, nil
}

// Get the total number of messages in an IMAP mailbox
func (m *DummyMailstore) TotalMessages(mbox int64) (int64, error) {
	return 8, nil
}

// Get the total number of unread messages in an IMAP mailbox
func (m *DummyMailstore) RecentMessages(mbox int64) (int64, error) {
	return 4, nil
}

// Get the next available uid in an IMAP mailbox
func (m *DummyMailstore) NextUid(mbox int64) (int64, error) {
	return 9, nil
}
