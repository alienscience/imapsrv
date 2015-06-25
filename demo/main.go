package main

import (
	imap "github.com/alienscience/imapsrv"
	"log"
)

func main() {
	// The simplest possible server - zero config
	// It will start a server on port 143
	//s := imap.NewServer()
	//s.Start()

	// More advanced config
	m := &imap.DummyMailstore{}

	s := imap.NewServer(
		imap.Listen("127.0.0.1:1193"),
		imap.Listen("127.0.0.1:1194"),
		imap.Store(m),
	)

	err := s.Start()
	if err != nil {
		log.Print("IMAP server not started")
	}
}
