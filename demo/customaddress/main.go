package main

import (
	imap "github.com/alienscience/imapsrv"
	"log"
)

func main() {
	// This server listens on two different ports, and with a non-default Mailstore
	m := &imap.DummyMailstore{}

	s := imap.NewServer(
		imap.ListenOption("127.0.0.1:1193"),
		imap.ListenOption("127.0.0.1:1194"),
		imap.StoreOption(m),
	)

	err := s.Start()
	if err != nil {
		log.Print("IMAP server not started")
	}
}
