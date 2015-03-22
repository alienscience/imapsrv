package imapsrv

import (
	"github.com/jhillyerd/go.enmime"
	"net/mail"
)

// A wrapper around a Mailbox that provides helper functions
// and Sequence numbers
type mailboxWrap struct {
	// The mailbox
	provider Mailbox
	// Sequence number to uid mapping
	seqNums []int32
}

// Get a mailbox from a mailstore
func getMailbox(store Mailstore, path []string) (*mailboxWrap, error) {
	mbox, err := store.Mailbox(path)
	if err != nil {
		return nil, err
	}

	return wrapMailbox(mbox), nil
}

// Get mailboxes from a mailstore
func getMailboxes(store Mailstore, path []string) ([]*mailboxWrap, error) {
	mboxes, err := store.Mailboxes(path)
	if err != nil {
		return nil, err
	}

	ret := make([]*mailboxWrap, len(mboxes))
	for i, mbox := range mboxes {
		ret[i] = wrapMailbox(mbox)
	}

	return ret, nil
}

// Wrap a Mailbox returned by the mailstore
func wrapMailbox(mbox Mailbox) *mailboxWrap {
	return &mailboxWrap{
		provider: mbox,
	}
}

// Fetch the message from the mailbox with the given sequence number
func (m *mailboxWrap) fetch(seqnum int32) (*enmime.MIMEBody, error) {

	uid, err := m.getUid(seqnum)
	if err != nil {
		return nil, err
	}

	// Get a reader to read the message
	reader, err := m.provider.Fetch(uid)
	if err != nil {
		return nil, err
	}

	// For now read the whole message into memory
	msg, err := mail.ReadMessage(reader)
	if err != nil {
		return nil, err
	}

	ret, err := enmime.ParseMIMEBody(msg)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// Get the Uid for the given sequence number
func (m *mailboxWrap) getUid(seqnum int32) (int32, error) {

	// Build the sequence number array
	if m.seqNums == nil {
		uids, err := m.provider.AllUids()
		if err != nil {
			return -1, err
		}

		m.seqNums = make([]int32, len(uids))

		for i, uid := range uids {
			m.seqNums[i] = uid
		}
	}

	// Return the UID
	return m.seqNums[seqnum], nil

}
