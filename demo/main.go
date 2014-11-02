
package main

import (
	imap "github.com/alienscience/imapsrv"
)

// A dummy mailstore used for demonstrating the IMAP server
type Mailstore struct {
}

func main() {
	// Configure an IMAP server on localhost port 1193
	config := imap.DefaultConfig()
	config.Interface = "127.0.0.1:1193"

	// Configure a dummy mailstore
	mailstore := &Mailstore{}
	config.Store = mailstore

	// Start the server
	server := imap.Create(config)
	server.Start()

}

//------ Dummy Mailstore -------------------------------------------------------

// Get mailbox information
func (m *Mailstore) GetMailbox(name string) (*imap.Mailbox, error) {
	return &imap.Mailbox{
		Name: "inbox",
		Id: 1,
	}, nil
}

// Get the sequence number of the first unseen message
func (m *Mailstore) FirstUnseen(mbox int64) (int64, error) {
	return 4, nil
}

// Get the total number of messages in an IMAP mailbox
func (m *Mailstore) TotalMessages(mbox int64) (int64, error) {
	return 8, nil
}

// Get the total number of unread messages in an IMAP mailbox
func (m *Mailstore) RecentMessages(mbox int64) (int64, error) {
	return 4, nil
}

// Get the next available uid in an IMAP mailbox
func (m *Mailstore) NextUid(mbox int64) (int64, error) {
	return 9, nil
}
