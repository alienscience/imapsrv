[![Stories in Ready](https://badge.waffle.io/alienscience/imapsrv.png?label=ready&title=Ready)](https://waffle.io/alienscience/imapsrv)

# Imapsrv

This is an IMAP server written in Go. It is a work in progress.

# Demo

In the demo subdirectory there are several implementation examples available. 

```
$ go run ./demo/basic/main.go
```

You can connect to this server using telnet or netcat. For example:

```
$ nc -C localhost 1193
* OK IMAP4rev1 Service Ready
10 LOGIN test anypassword
10 OK LOGIN completed
20 CAPABILITY
* CAPABILITY IMAP4rev1
20 OK CAPABILITY completed
30 SELECT inbox
* 8 EXISTS
* 4 RECENT
* OK [UNSEEN 4] Message 4 is first unseen
* OK [UIDVALIDITY 1] UIDs valid
* OK [UIDNEXT 9] Predicted next UID
30 OK SELECT completed
40 LOGOUT
* BYE IMAP4rev1 Server logging out
40 OK LOGOUT completed
```

# Developing

The server is not fully operational on its own. It requires a mailstore and an authentication mechanism. 

It defines an interface in mailstore.go which describes the service it needs from a Mailstore. For example a Mailstore could serve its data from: database, filesystem, maildir, etc...
At the moment only one mailstore can be used at the same time.

To add a new IMAP command the usual steps are:

1. Add the command to parser.go
2. Add the command and its client interaction to commands.go
3. Put the main functionality in session.go.

## Current state
### IMAP ([RFC 3501](https://tools.ietf.org/html/rfc3501))
### Client Commands - Any state
- [x] CAPABILITY command
- [x] NOOP command
- [x] LOGOUT command

### Client Commands - Not-Authenticated State
- [x] STARTTLS command
- [x] AUTHENTICATE command
- [x] LOGIN command

### Client Commands - Authenticated State
- [x] SELECT command
- [x] EXAMINE command
- [ ] CREATE command - in progress (not recursive yet)
- [x] DELETE command
- [ ] RENAME command - in progress (special case INBOX missing)
- [X] SUBSCRIBE command
- [X] UNSUBSCRIBE command
- [x] LIST command
- [X] LSUB command
- [ ] STATUS command
- [ ] APPEND command

### Client Commands - Selected State
- [ ] CHECK command
- [ ] CLOSE command
- [ ] EXPUNGE command
- [ ] SEARCH command
- [ ] FETCH command - in progress
- [ ] STORE command
- [ ] COPY command
- [ ] UID command

### Server responses
- [x] OK response
- [x] NO response
- [x] BAD response
- [ ] PREAUTH response
- [x] BYE response

# License

3-clause BSD
