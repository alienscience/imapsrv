package imapsrv

import (
	"io"
	"time"
)

// A service that is needed to read mail messages
type Mailstore interface {
	// Get IMAP mailbox information
	// Returns nil if the mailbox does not exist
	Mailbox(path []string) (Mailbox, error)
	// Get a list of mailboxes at the given path
	Mailboxes(path []string) ([]Mailbox, error)
	// NewMessage adds the raw message information to the server, in the correct location
	NewMessage(message io.Reader) (Message, error)
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
	Fetch(uid int32) (Message, error)
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

// A message is read through this interface
type MessageReader interface {
	io.Reader
	io.Seeker
	io.Closer
}

// An IMAP message
type Message interface {
	// Get the message flags
	Flags() (uint8, error)
	// Get the date of the message as known by the server
	InternalDate() (time.Time, error)
	// Get the size of the message in bytes
	Size() (uint32, error)
	// Get a reader to access the message content
	Reader() (MessageReader, error)
}

// Message flags
const (
	// The message has been read
	Seen = 1 << iota
	// The message has been answered
	Answered
	// The message has been flagged for urgent/special attention
	Flagged
	// The message has been marked for removal by EXPUNGE
	Deleted
	// The message is imcomplete and is being worked on
	Draft
	// The message has recently arrived in the mailbox
	Recent
)

var messageFlags = map[uint8]string{
	Seen:     `\Seen`,
	Answered: `\Answered`,
	Flagged:  `\Flagged`,
	Deleted:  `\Deleted`,
	Draft:    `\Draft`,
	Recent:   `\Recent`,
}
