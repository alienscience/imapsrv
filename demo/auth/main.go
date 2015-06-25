package main

import (
	imap "github.com/alienscience/imapsrv"
	"github.com/alienscience/imapsrv/auth"
	"io/ioutil"
	"log"
)

func main() {
	// This server uses boltDb for its authentication, adding a test user
	m := &imap.DummyMailstore{}

	// Create a file for the BoltAuthStore - in production this should probably NOT be a temporary file (!)
	tmpFile, err := ioutil.TempFile("", "imap_")
	if err != nil {
		log.Fatalln("Could not create tempfile:", err)
	}

	// Initialize authentication backend
	a, err := auth.NewBoltAuthStore(tmpFile.Name())
	if err != nil {
		log.Fatalln("Could not create BoltAuthStore:", err)
	}

	// Add a user
	a.CreateUser("test@example.com", "password")

	// Put everything together
	s := imap.NewServer(
		imap.ListenOption("127.0.0.1:1193"),
		imap.StoreOption(m),
		imap.AuthStoreOption(a),
	)

	// Firing up the server
	err = s.Start()
	if err != nil {
		log.Print("IMAP server not started")
	}
}
