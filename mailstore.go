package imapsrv

import (
	"io"
)

// A service that is needed to read mail messages
type Mailstore interface {
	// Get IMAP mailbox information
	// Returns nil if the mailbox does not exist
	Mailbox(path []string) (Mailbox, error)
	// Get a list of mailboxes at the given path
	Mailboxes(path []string) ([]Mailbox, error)
}

// An IMAP mailbox
// The mailbox must be able to handle uids. Sequence numbers are handled by imapsrv.
type Mailbox interface {
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
	Fetch(uid int32) (io.ReadSeeker, error)
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

