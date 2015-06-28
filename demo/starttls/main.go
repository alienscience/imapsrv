package main

import (
	"fmt"
	imap "github.com/alienscience/imapsrv"
	"log"
)

func main() {
	// This server listens on two different ports, one of which allows STARTTLS

	s := imap.NewServer(
		imap.ListenOption("127.0.0.1:1193"), // optionally also listen to non-STARTTLS ports
		imap.ListenSTARTTLSOoption("127.0.0.1:1194", "demo/starttls/public.pem", "demo/starttls/private.pem"),
	)

	fmt.Println("Starting server, you can test by doing:\n",
		"$ telnet localhost 1193\n",
		"or\n",
		"$ openssl s_client -starttls imap -crlf -connect 'localhost:1194'")

	err := s.Start()
	if err != nil {
		log.Print("IMAP server not started")
	}
}
