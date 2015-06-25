// An IMAP server
package imapsrv

import (
	"bufio"
	"crypto/tls"
	"fmt"
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
}

// Listener config
type listener struct {
	addr         string
	tls          bool
	certificates []tls.Certificate
	listener     net.Listener
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
	// conn is the lowest-level connection layer
	conn net.Conn
	// listener refers to the listener that's handling this client
	listener listener

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
func Store(m Mailstore) func(*Server) error {
	return func(s *Server) error {
		s.config.mailstore = m
		return nil
	}
}

// Add an interface to listen to
func Listen(Addr string) func(*Server) error {
	return func(s *Server) error {
		l := listener{
			addr: Addr,
		}
		s.config.listeners = append(s.config.listeners, l)
		return nil
	}
}

func ListenSTARTTLS(Addr, certFile, keyFile string) func(*Server) error {
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
			tls:          true,
			certificates: certs,
		}
		s.config.listeners = append(s.config.listeners, l)
		return nil
	}
}

// Set MaxClients config
func MaxClients(max uint) func(*Server) error {
	return func(s *Server) error {
		s.config.maxClients = max
		return nil
	}
}

func NewServer(options ...func(*Server) error) *Server {
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
	}

	// Start the server on each port
	n := len(s.config.listeners)
	for i := 0; i < n; i += 1 {
		listener := s.config.listeners[i]

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
func (s *Server) runListener(listener listener, id int) {

	log.Printf("IMAP server %d listening on %s", id, listener.listener.Addr().String())

	clientNumber := 1

	for {
		// Accept a connection from a new client
		conn, err := listener.listener.Accept()
		if err != nil {
			log.Print("IMAP accept error, ", err)
			continue
		}

		// Handle the client
		client := &client{
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
	sess := createSession(c.id, c.config, s, &c.listener, c.conn)

	for {
		// Get the next IMAP command
		command := parser.next()

		// Execute the IMAP command
		response := command.execute(sess)

		// Possibly replace buffers (layering)
		if response.bufOutReplacement != nil {
			c.bufout = response.bufOutReplacement
		}
		if response.bufInReplacement != nil {
			c.bufin = response.bufInReplacement
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

// Close an IMAP client
func (c *client) close() {
	c.conn.Close()
}

// Log an error
func (c *client) logError(err error) {
	log.Printf("IMAP client %s, %v", c.id, err)
}
