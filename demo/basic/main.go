package main

import imap "github.com/alienscience/imapsrv"

func main() {
	// The simplest possible server - zero config
	// It will start a server on port 143
	s := imap.NewServer()
	s.Start()
}
