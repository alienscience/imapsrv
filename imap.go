// An IMAP server
package imapsrv

import (
	"bufio"
	"fmt"
	"github.com/EtienneBruines/imapsrv/auth"
	"log"
	"net"
)

// Default listen interface/port
const DefaultListener = "0.0.0.0:143"

// IMAP server configuration
type config struct {
	maxClients uint
	listeners  []listener
	mailstore  Mailstore

	authBackend auth.AuthStore
}

type option func(*Server) error

// Listener config
type listener struct {
	addr string
}

// An IMAP Server
type Server struct {
	// Server configuration
	config *config
	// Number of active clients
	activeClients uint
}

// An IMAP Client as seen by an IMAP server
type client struct {
	conn   net.Conn
	bufin  *bufio.Reader
	bufout *bufio.Writer
	id     string
	config *config
}

// Return the default server configuration
func defaultConfig() *config {
	return &config{
		listeners:  make([]listener, 0, 4),
		maxClients: 8,
	}
}

// Add a mailstore to the config
func StoreOption(m Mailstore) option {
	return func(s *Server) error {
		s.config.mailstore = m
		return nil
	}
}

func AuthStoreOption(a auth.AuthStore) option {
	return func(s *Server) error {
		s.config.authBackend = a
		return nil
	}
}

// Add an interface to listen to
func ListenOption(Addr string) option {
	return func(s *Server) error {
		l := listener{
			addr: Addr,
		}
		s.config.listeners = append(s.config.listeners, l)
		return nil
	}
}

// Set MaxClients config
func MaxClientsOption(max uint) option {
	return func(s *Server) error {
		s.config.maxClients = max
		return nil
	}
}

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

	listeners := make([]net.Listener, 0, 4)

	// Use a default listener if none exist
	if len(s.config.listeners) == 0 {
		s.config.listeners = append(s.config.listeners,
			listener{addr: DefaultListener})
	}

	// Start listening for IMAP connections
	for _, iface := range s.config.listeners {
		listener, err := net.Listen("tcp", iface.addr)
		if err != nil {
			log.Printf("IMAP cannot listen on %s, %v", iface.addr, err)
			return err
		}

		listeners = append(listeners, listener)
	}

	// Start the server on each port
	n := len(listeners)
	for i := 0; i < n; i += 1 {
		listener := listeners[i]

		// Start each listener in a separate go routine
		// except for the last one
		if i < n-1 {
			go s.runListener(listener, i)
		} else {
			s.runListener(listener, i)
		}
	}

	return nil
}

// Run a listener
func (s *Server) runListener(listener net.Listener, id int) {

	log.Printf("IMAP server %d listening on %s", id, listener.Addr().String())

	clientNumber := 1

	for {
		// Accept a connection from a new client
		conn, err := listener.Accept()
		if err != nil {
			log.Print("IMAP accept error, ", err)
			continue
		}

		// Handle the client
		client := &client{
			conn:   conn,
			bufin:  bufio.NewReader(conn),
			bufout: bufio.NewWriter(conn),
			id:     fmt.Sprint(id, "/", clientNumber),
			config: s.config,
		}

		go client.handle(s)

		clientNumber += 1
	}

}

// Handle requests from an IMAP client
func (c *client) handle(s *Server) {

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
	sess := createSession(c.id, c.config, s)

	for {
		// Get the next IMAP command
		command := parser.next()

		// Execute the IMAP command
		response := command.execute(sess)

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

// Close an IMAP client
func (c *client) close() {
	c.conn.Close()
}

// Log an error
func (c *client) logError(err error) {
	log.Printf("IMAP client %s, %v", c.id, err)
}
