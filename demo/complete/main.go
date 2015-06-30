package main

import (
	imap "github.com/alienscience/imapsrv"
	"github.com/alienscience/imapsrv/auth/boltstore"
	"github.com/alienscience/imapsrv/mailstore/boltmail"
	"io/ioutil"
	"log"
)

func main() {
	// This server uses everything from other packages, combined

	// Create a file for the BoltAuthStore - in production this should probably NOT be a temporary file (!)
	tmpFile, err := ioutil.TempFile("", "imap_")
	if err != nil {
		log.Fatalln("Could not create tempfile:", err)
	}
	// Initialize authentication backend
	authStore, err := boltstore.NewBoltAuthStore(tmpFile.Name() + "_authstore")
	if err != nil {
		log.Fatalln("Could not create BoltAuthStore:", err)
	}
	// Add a user
	authStore.CreateUser("test@example.com", "password")

	mailStore, err := boltmail.NewBoltMailstore(tmpFile.Name() + "_mailstore")
	if err != nil {
		log.Fatalln("Could not create BoltMailstore:", err)
	}

	// Put everything together
	s := imap.NewServer(
		// AUTH
		imap.AuthStoreOption(authStore),
		// Mailstore
		imap.StoreOption(mailStore),
		// IMAP
		imap.ListenOption("127.0.0.1:1193"),
		imap.ListenSTARTTLSOoption("127.0.0.1:1194", "demo/certificates/public.pem", "demo/certificates/private.pem"),
		// LMTP
		imap.LMTPOptionSocket("/tmp/imapsrv-lmtp"),
		imap.LMTPOptionTCP("localhost:61194"),
	)

	// Firing up the server
	err = s.Start()
	if err != nil {
		log.Print("IMAP server not started")
	}
}
