
# Imapsrv

This is an IMAP server written in Go. It is a work in progress.

# Demo

In the demo subdirectory is an example IMAP server that starts up on port 1193. To run this server:

```
$ cd demo
$ go build
$ ./demo
```

You can connect to this server using telnet or netcat. For example:

```
$ nc -C localhost 1193
* OK IMAP4rev1 Service Ready
* LOGIN test anypassword
* OK LOGIN completed
* CAPABILITY
* CAPABILITY IMAP4rev1
* OK CAPABILITY completed
* SELECT inbox
* 8 EXISTS
* 4 RECENT
* OK [UNSEEN 4] Message 4 is first unseen
* OK [UIDVALIDITY 1] UIDs valid
* OK [UIDNEXT 9] Predicted next UID
* OK SELECT completed
* LOGOUT
* BYE IMAP4rev1 Server logging out
* OK LOGOUT completed
```

# Developing

The server is not fully operational on its own. It defines an interface in session.go which describes the service it needs from a Mailstore. The Mailstore can be a database or filesystem or combination of both. The IMAP server does not care and it is up to the user to supply this service. 

To add a new IMAP command the usual steps are:

1. Add the command to parser.go
2. Add the command and its client interaction to commands.go
3. Put the main functionality in session.go.


# License

3-clause BSD
