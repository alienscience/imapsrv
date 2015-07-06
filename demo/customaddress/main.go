package main

import (
	"log"

	imap "github.com/alienscience/imapsrv"
)

func main() {
	// This server listens on two different ports

	s := imap.NewServer(
		imap.ListenOption("127.0.0.1:1193"),
		imap.ListenOption("127.0.0.1:1194"),
	)

	err := s.Start()
	if err != nil {
		log.Print("IMAP server not started")
	}
}
