package main

import (
	imap "github.com/alienscience/imapsrv"
)

func main() {
	// This package allows to receive e-mail using the LMTP protocol,
	// and allowing STARTTLS to connect to the imap server.

	lmtp := imap.LMTPOption("/tmp/imapsrv-lmtp")

	s := imap.NewServer(
		imap.ListenSTARTTLSOoption("127.0.0.1:1194", "demo/certificates/public.pem", "demo/certificates/private.pem"),
		lmtp,
	)
	s.Start()
}
