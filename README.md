
# Imapsrv

This is an IMAP server written in Go. It is a work in progress.

# Demo

In the demo subdirectory is an example IMAP server that starts up on port 1193. To run this server:

```
$ go run ./demo/main.go
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

The server is not fully operational on its own. It requires a mailstore and an authentication mechanism. 

It defines an interface in mailstore.go which describes the service it needs from a Mailstore. You can use multiple mailstores at the same time: database, filesystem, maildir, etc...

There are plans to add basic support for maildir and a basic database storage.

To add a new IMAP command the usual steps are:

1. Add the command to parser.go
2. Add the command and its client interaction to commands.go
3. Put the main functionality in session.go.


# License

3-clause BSD
