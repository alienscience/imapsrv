package main

import (
	imap "github.com/alienscience/imapsrv"
)

func main() {
	// This package allows to receive e-mail using the LMTP protocol,
	// and allowing STARTTLS to connect to the IMAP server.

	// Any of these two will do:
	sock := imap.LMTPOptionSocket("/tmp/imapsrv-lmtp")
	tcp := imap.LMTPOptionTCP("localhost:61194")

	s := imap.NewServer(
		imap.ListenSTARTTLSOoption("127.0.0.1:1194", "demo/certificates/public.pem", "demo/certificates/private.pem"),
		sock,
		tcp,
	)
	s.Start()
}
