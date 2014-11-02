
# Imapsrv

This is an IMAP server written in Go. It is a work in progress.

# Overview

The server is not fully operational on its own. It defines an interface in session.go which describes the service it needs from a Mailstore. The Mailstore can be a database or filesystem or combination of both. The IMAP server does not care and it is up to the user to supply this service. 

At the moment it is possible to start the server and use telnet to login/logout and get capabilities. To add a new IMAP command the usual steps are:

1. Add the command to parser.go
2. Add the command and its client interaction to commands.go
3. Put the main functionality in session.go.

# License

3-clause BSD
