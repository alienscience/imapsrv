package imapsrv

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/textproto"
	"strings"
)

const (
	unixNetwork = "unix"
	tcpNetwork  = "tcp"
)

type lmtpSession struct {
	receivingData bool

	rw *textproto.Conn

	recipients []string
	from       string

	closing bool
}

type lmtpClient struct {
	// conn is the lowest-level connection layer
	innerConn net.Conn
	// id can together with the listener-id be used to uniquely identify this connection
	id string
	// session holds important information about this current session
	session *lmtpSession
}

type lmtpEntryPoint struct {
	unix bool
	addr string
}

func LMTPOptionSocket(loc string) func(*Server) error {
	return func(s *Server) error {
		s.config.LmtpEndpoints = append(s.config.LmtpEndpoints, lmtpEntryPoint{true, loc})
		return nil
	}
}

func LMTPOptionTCP(addr string) func(*Server) error {
	return func(s *Server) error {
		s.config.LmtpEndpoints = append(s.config.LmtpEndpoints, lmtpEntryPoint{false, addr})
		return nil
	}
}

func (s *Server) runLMTPListener(entrypoint lmtpEntryPoint, number int) {
	// Add hostname to responses
	lmtpStatusReady = fmt.Sprintf(lmtpStatusReady, s.config.Hostname)
	lmtpStatusClosing = fmt.Sprintf(lmtpStatusClosing, s.config.Hostname)
	lmtpStatusLHLO = fmt.Sprintf(lmtpStatusLHLO, s.config.Hostname)

	var listener net.Listener

	if entrypoint.unix {
		// Parse unix address
		addr, err := net.ResolveUnixAddr(unixNetwork, entrypoint.addr)
		if err != nil {
			log.Println("Warning, LMTP endpoint", number, "not started:", err)
		}

		// Create unix socket
		listener, err = net.ListenUnix(unixNetwork, addr)
		if err != nil {
			log.Println("Warning, LMTP endpoint", number, "not started:", err)
		}
	} else {
		// Parse tcp address
		addr, err := net.ResolveTCPAddr(tcpNetwork, entrypoint.addr)
		if err != nil {
			log.Println("Warning, LMTP endpoint", number, "not started:", err)
		}

		// Create tcp socket
		listener, err = net.ListenTCP(tcpNetwork, addr)
		if err != nil {
			log.Println("Warning, LMTP endpoint", number, "not started:", err)
		}
	}

	defer listener.Close()
	s.socketsMu.Lock()
	s.sockets = append(s.sockets, listener)
	s.socketsMu.Unlock()

	// Remove unix socket when we are asked to close, since defer won't be called

	log.Printf("LMTP entrypoint %d available at %s", number, entrypoint.addr)

	clientNumber := 0
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Warning: accepting failed:", err)
			continue
		}

		// Handle the client
		client := &lmtpClient{
			innerConn: conn,
			id:        fmt.Sprintf("%s / %s", number, clientNumber),
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
			log.Println("Panic:", e)
		}
	}()

	c.session = &lmtpSession{
		rw: textproto.NewConn(c.innerConn),
	}

	// Write the welcome message
	writeSimpleLine(lmtpStatusReady, c.session.rw.W)

	for {
		if c.session.receivingData {
			// Read everything until <CRLF>.<CRLF>
			data, err := c.session.rw.ReadDotBytes()
			if err != nil {
				log.Println("Error while reading raw data:", err)
				writeSimpleLine(lmtpStatusProcessingError, c.session.rw.W)
				continue
			}

			c.session.receivingData = false
			writeSimpleLine("250 OK", c.session.rw.W)

			// TODO: we should now process the session,
			// before continuing (or else it gets lost forever)
			log.Println(string(data))

		} else {
			// Read line, tags, process and respond
			line, err := c.session.rw.ReadLine()
			if err != nil {
				log.Println(err)
			}

			tags := strings.Fields(line)
			output := c.session.process(tags, line)

			writeSimpleLine(output, c.session.rw.W)

			if c.session.closing {
				return // thereby closing the connection
			}
		}
	}
}

var (
	lmtpStatusReady           = "220 %s LMTP server ready"
	lmtpStatusClosing         = "221 %s closing connection"
	lmtpStatusOK              = "250 OK"
	lmtpStatusLHLO            = "250 %s"
	lmtpStatusDataStart       = "354 Start mail input; end with <CRLF>.<CRLF>"
	lmtpStatusProcessingError = "451 Requested action aborted: error in processing"
	lmtpStatusUnknown         = "500 command unrecognised"
	lmtpStatusSyntaxArgs      = "501 Syntax error in parameters or arguments"
	lmtpStatusBadSeq          = "503 Bad sequence of commands"
)

func (sess *lmtpSession) process(tags []string, line string) string {
	switch strings.ToUpper(tags[0]) {
	case "LHLO":
		return lmtpStatusLHLO

	case "MAIL":
		start := strings.IndexRune(line, '<')
		end := strings.IndexRune(line, '>')
		if start < 6 || end < start {
			return lmtpStatusSyntaxArgs
		}
		sess.from = line[start+1 : end]
		return lmtpStatusOK

	case "RCPT":
		start := strings.IndexRune(line, '<')
		end := strings.IndexRune(line, '>')
		if start < 6 || end < start {
			return lmtpStatusSyntaxArgs
		}
		sess.recipients = append(sess.recipients, line[start+1:end])
		return lmtpStatusOK

	case "QUIT":
		sess.closing = true
		return lmtpStatusClosing

	case "DATA":
		if len(sess.recipients) == 0 {
			return lmtpStatusBadSeq
		}
		sess.receivingData = true
		return lmtpStatusDataStart

	default:
		log.Println("idk; received:", line)
		return lmtpStatusUnknown
	}
}

func writeSimpleLine(mes string, w *bufio.Writer) {
	_, err := w.WriteString(mes + "\r\n")
	if err != nil {
		log.Println("Error while writing", err)
	}
	err = w.Flush()
	if err != nil {
		log.Println("Error while flushing:", err)
	}
}

// close closes an LMTP client
func (c *lmtpClient) close() {
	defer c.innerConn.Close()
	c.session.rw.Close()
}
