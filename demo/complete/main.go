package main

import (
	"log"

	"os"

	imap "github.com/alienscience/imapsrv"
	"github.com/alienscience/imapsrv/auth/boltstore"
	"github.com/alienscience/imapsrv/mailstore/boltmail"
)

func main() {
	// This server uses everything from other packages, combined

	// Create a file for the BoltAuthStore - in production this should probably NOT be a temporary file (!)
	tmpFile, err := os.Create("/tmp/imapsrv_db")
	if err != nil {
		log.Fatalln("Could not create tempfile:", err)
	}
	// Initialize authentication backend
	authStore, err := boltstore.NewBoltAuthStore(tmpFile.Name() + "_authstore")
	if err != nil {
		log.Fatalln("Could not create BoltAuthStore:", err)
	}

	// Initialize mailstorage backend
	mailStore, err := boltmail.NewBoltMailstore(tmpFile.Name() + "_mailstore")
	if err != nil {
		log.Fatalln("Could not create BoltMailstore:", err)
	}

	// Add a user
	authStore.CreateUser("test@example.local", "password")
	mailStore.NewUser("test@example.local")
	mailStore.NewMailbox("test@example.local", []string{"INBOX"})
	mailStore.NewMailbox("test@example.local", []string{"Trash"})

	// Put everything together
	s := imap.NewServer(
		// AUTH
		imap.AuthStoreOption(authStore),
		// Mailstore
		imap.StoreOption(mailStore),
		// IMAP
		imap.ListenSTARTTLSOoption("127.0.0.1:1194", "demo/certificates/public.pem", "demo/certificates/private.pem"),
		// LMTP
		imap.LMTPOptionTCP("localhost:61194"),
	)

	// Firing up the server
	err = s.Start()
	if err != nil {
		log.Print("IMAP server not started")
	}
}
