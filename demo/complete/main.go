package main

import (
	imap "github.com/alienscience/imapsrv"
	"github.com/alienscience/imapsrv/auth/boltstore"
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
	a, err := boltstore.NewBoltAuthStore(tmpFile.Name())
	if err != nil {
		log.Fatalln("Could not create BoltAuthStore:", err)
	}
	// Add a user
	a.CreateUser("test@example.com", "password")

	// Put everything together
	s := imap.NewServer(
		// AUTH
		imap.AuthStoreOption(a),
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
