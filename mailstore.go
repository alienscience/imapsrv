package imapsrv

// A service that is needed to read mail messages
type Mailstore interface {
	// Get IMAP mailbox information
	// Returns nil if the mailbox does not exist
	GetMailbox(name string) (*Mailbox, error)
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
func (m *DummyMailstore) GetMailbox(name string) (*Mailbox, error) {
	return &Mailbox{
		Name: "inbox",
		Id:   1,
	}, nil
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
