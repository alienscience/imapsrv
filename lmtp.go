package imapsrv

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"net/textproto"
	"strings"
)

const (
	unixNetwork = "unix"
)

type lmtpSession struct {
	receivingData bool
	data          bytes.Buffer

	rw *textproto.Conn

	recipients []string
	from       string
}

type lmtpClient struct {
	// conn is the lowest-level connection layer
	innerConn net.Conn
	// listener refers to the listener that's handling this client
	listener *net.UnixListener

	bufin  *bufio.Reader
	bufout *bufio.Writer
	id     string
	config *config

	session *lmtpSession
}

func LMTPOption(entrypoint string) func(*Server) error {
	return func(s *Server) error {
		s.config.lmtpEndpoints = append(s.config.lmtpEndpoints, entrypoint)
		return nil
	}
}

func (s *Server) runLMTPListener(entrypoint string, number int) {
	// TODO: what if someone wants the entrypoint to be a address + port?

	addr, err := net.ResolveUnixAddr(unixNetwork, entrypoint)
	if err != nil {
		log.Fatalln(err) // TODO: do we want to crash fatally? Does this also crash other goroutines?
	}

	// TODO: stop listening when this all goes south
	unixListener, err := net.ListenUnix(unixNetwork, addr)
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("LMTP entrypoint %d available at %s", number, entrypoint)

	clientNumber := 0
	for {
		conn, err := unixListener.AcceptUnix()
		if err != nil {
			log.Println("Warning: accepting failed:", err)
			continue
		}
		log.Println("Got connection")

		// Handle the client
		client := &lmtpClient{
			innerConn: conn,
			listener:  unixListener,
			bufin:     bufio.NewReader(conn),
			bufout:    bufio.NewWriter(conn),
			// TODO: perhaps we can do this without Sprint, maybe strconv.Itoa()
			id:     fmt.Sprint(number, "/", clientNumber),
			config: s.config,
		}

		go client.handle(s)

		clientNumber += 1
	}
}

func (c *lmtpClient) handle(s *Server) {
	// Close the client on exit from this function
	defer c.close()

	// Handle parser panics gracefully
	defer func() {
		if e := recover(); e != nil {
			log.Println("Panic received:", e)
		}
	}()

	c.session = &lmtpSession{
		rw: textproto.NewConn(c.innerConn),
	}

	// Write the welcome message
	err := writeLine("220 deskserver.local LMTP server ready", c.session.rw.W)
	if err != nil {
		log.Println("Error while writing:", err)
		return
	}

	for {
		line, err := c.session.rw.ReadLine()
		if err != nil {
			log.Println(err)
		}
		tags := strings.Fields(line)

		if c.session.receivingData {
			if line == "." {
				c.session.receivingData = false
				log.Println("DATA:", c.session.data.String())
				writeOK(c.session.rw.W)
				// TODO: we should now process the session, before continuing
				continue
			}
			c.session.data.WriteString(line)
			c.session.data.WriteRune('\r')
			c.session.data.WriteRune('\n')

		} else {
			switch strings.ToUpper(tags[0]) {
			case "LHLO":
				err = writeLine("250 deskserver.local", c.session.rw.W)
				if err != nil {
					log.Println("Error while sending LHLO response", err)
					return
				}

			case "MAIL":
				start := strings.IndexRune(line, '<')
				end := strings.IndexRune(line, '>')
				if start < 6 || end < start {
					writeErrorArgs(c.session.rw.W)
					continue
				}
				c.session.from = line[start+1 : end]
				writeOK(c.session.rw.W)

			case "RCPT":
				start := strings.IndexRune(line, '<')
				end := strings.IndexRune(line, '>')
				if start < 6 || end < start {
					writeErrorArgs(c.session.rw.W)
					continue
				}
				c.session.recipients = append(c.session.recipients, line[start+1:end])
				writeOK(c.session.rw.W)

			case "QUIT":
				err = writeLine("221 deskserver.local closing connection", c.session.rw.W)
				if err != nil {
					log.Println("Error while sending QUIT response", err)
				}
				return // closes because of defer

			case "DATA":
				if len(c.session.recipients) == 0 {
					writeLine("503 Bad sequence of commands", c.session.rw.W)
					continue
				}
				writeLine("354 Start mail input; end with <CRLF>.<CRLF>", c.session.rw.W)
				c.session.receivingData = true

			default:
				log.Println("idk; received:", line)
				writeLine("500 command unrecognised", c.session.rw.W)
			}
		}
	}
}

func writeOK(w *bufio.Writer) {
	err := writeLine("250 OK", w)
	if err != nil {
		log.Println("Error while writing OK:", err)
	}
}

func writeErrorArgs(w *bufio.Writer) {
	err := writeLine("501 Syntax error in parameters or arguments", w)
	if err != nil {
		log.Println("Error while writing 501:", err)
	}
}

func writeLine(mes string, w *bufio.Writer) error {
	_, err := w.WriteString(mes + "\r\n")
	if err != nil {
		return err
	}
	err = w.Flush()
	if err != nil {
		return err
	}

	return nil
}

// close closes an LMTP client
func (c *lmtpClient) close() {
	defer c.innerConn.Close()
	c.session.rw.Close()
}

// logError sends a log message to the default Logger
func (c *lmtpClient) logError(err error) {
	log.Printf("LMTP client %s, %v", c.id, err)
}
