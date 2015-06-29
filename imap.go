// An IMAP server
package imapsrv

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"github.com/alienscience/imapsrv/auth"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// DefaultListener is the listener that is used if no listener is specified
const DefaultListener = "0.0.0.0:143"

// config is an IMAP server configuration
type config struct {
	maxClients uint
	listeners  []listener
	mailstore  Mailstore

	authBackend auth.AuthStore

	lmtpEndpoints []lmtpEntryPoint

	// hostname is the hostname of this entire server
	hostname string
}

type option func(*Server) error

// listener represents a listener as used by the server
type listener struct {
	addr         string
	encryption   encryptionLevel
	certificates []tls.Certificate
	listener     net.Listener
}

// Server is an IMAP Server
type Server struct {
	// Server configuration
	config *config
	// Number of active clients
	activeClients uint
	// sockets
	sockets []net.Listener
	// socketsMu ensures that multiple goroutines can access the sockets list
	socketsMu sync.Mutex
}

// client is an IMAP Client as seen by an IMAP server
type imapClient struct {
	// conn is the lowest-level connection layer
	conn net.Conn
	// listener refers to the listener that's handling this client
	listener listener

	bufin  *bufio.Reader
	bufout *bufio.Writer
	id     string
	config *config
}

// defaultConfig returns the default server configuration
func defaultConfig() *config {
	return &config{
		listeners:  make([]listener, 0, 4),
		maxClients: 8,
	}
}

// Add a mailstore to the config
// StoreOption add a mailstore to the config
func StoreOption(m Mailstore) option {
	return func(s *Server) error {
		s.config.mailstore = m
		return nil
	}
}

// AuthStoreOption adds an authenticaton backend
func AuthStoreOption(a auth.AuthStore) option {
	return func(s *Server) error {
		s.config.authBackend = a
		return nil
	}
}

// ListenOption adds an interface to listen to
func ListenOption(Addr string) option {
	return func(s *Server) error {
		l := listener{
			addr: Addr,
		}
		s.config.listeners = append(s.config.listeners, l)
		return nil
	}
}

// ListenSTARTTLSOoption enables STARTTLS with the given certificate and keyfile
func ListenSTARTTLSOoption(Addr, certFile, keyFile string) option {
	return func(s *Server) error {
		// Load the ceritificates
		var err error
		certs := make([]tls.Certificate, 1)
		certs[0], err = tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return err
		}

		// Set up the listener
		l := listener{
			addr:         Addr,
			encryption:   starttlsLevel,
			certificates: certs,
		}
		s.config.listeners = append(s.config.listeners, l)
		return nil
	}
}

// MaxClientsOption sets the MaxClients config
func MaxClientsOption(max uint) option {
	return func(s *Server) error {
		s.config.maxClients = max
		return nil
	}
}

// NewServer creates a new server with the given options
func NewServer(options ...option) *Server {
	// set the default config
	s := &Server{}
	s.config = defaultConfig()

	// override the config with the functional options
	for _, option := range options {
		err := option(s)
		if err != nil {
			panic(err)
		}
	}

	return s
}

// Start an IMAP server
func (s *Server) Start() error {
	// Use a default listener if none exist
	if len(s.config.listeners) == 0 {
		s.config.listeners = append(s.config.listeners,
			listener{addr: DefaultListener})
	}

	var err error
	// Start listening for IMAP connections
	for i, iface := range s.config.listeners {
		s.config.listeners[i].listener, err = net.Listen("tcp", iface.addr)
		if err != nil {
			log.Printf("IMAP cannot listen on %s, %v", iface.addr, err)
			return err
		}
		s.socketsMu.Lock()
		s.sockets = append(s.sockets, s.config.listeners[i].listener)
		s.socketsMu.Unlock()
	}

	// Start the LMTP entrypoints as desired
	for i, entrypoint := range s.config.lmtpEndpoints {
		go s.runLMTPListener(entrypoint, i)
	}

	// Start the server on each port
	for i, listener := range s.config.listeners {
		go s.runListener(listener, i)
	}

	// Wait for sigkill, so we can terminate this server
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	for range c {
		for _, sock := range s.sockets {
			sock.Close()
		}
		os.Exit(0)
	}

	return nil
}

// runListener runs the given listener on a separate goroutine
func (s *Server) runListener(listener listener, id int) {
	log.Printf("IMAP server %d listening on %s", id, listener.listener.Addr().String())

	clientNumber := 1

	for {
		// Accept a connection from a new client
		conn, err := listener.listener.Accept()
		if err != nil {
			log.Printf("IMAP accept error at %d, %v", id, err)
			continue
		}

		// Handle the client
		client := &imapClient{
			conn:     conn,
			listener: listener,
			bufin:    bufio.NewReader(conn),
			bufout:   bufio.NewWriter(conn),
			// TODO: perhaps we can do this without Sprint, maybe strconv.Itoa()
			id:     fmt.Sprint(id, "/", clientNumber),
			config: s.config,
		}

		go client.handle(s)

		clientNumber += 1
	}

}

// handle requests from an IMAP client
func (c *imapClient) handle(s *Server) {

	// Close the client on exit from this function
	defer c.close()

	// Handle parser panics gracefully
	defer func() {
		if e := recover(); e != nil {
			err := e.(parseError)
			c.logError(err)
			fatalResponse(c.bufout, err)
		}
	}()

	// Create a parser
	parser := createParser(c.bufin)

	// Write the welcome message
	err := ok("*", "IMAP4rev1 Service Ready").write(c.bufout)

	if err != nil {
		c.logError(err)
		return
	}

	//  Create a session
	sess := createSession(c.id, c.config, s, &c.listener, c.conn)

	for {
		// Get the next IMAP command
		command := parser.next()

		// Execute the IMAP command
		response := command.execute(sess)

		// Possibly replace buffers (layering)
		if response.bufReplacement != nil {
			c.bufout = response.bufReplacement.W
			c.bufin = response.bufReplacement.R
			parser.lexer.reader = &response.bufReplacement.Reader
		}

		// Write back the response
		err = response.write(c.bufout)

		if err != nil {
			c.logError(err)
			return
		}

		// Should the connection be closed?
		if response.closeConnection {
			return
		}
	}
}

// close closes an IMAP client
func (c *imapClient) close() {
	c.conn.Close()
}

// logError sends a log message to the default Logger
func (c *imapClient) logError(err error) {
	log.Printf("IMAP client %s, %v", c.id, err)
}
