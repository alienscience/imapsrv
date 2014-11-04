// An IMAP server
package imapsrv

import (
	"bufio"
	"log"
	"net"
)

// IMAP server configuration
type Config struct {
	MaxClients uint
	Listeners  []Listener
	Mailstores []Mailstore
}

// An IMAP Server
type Server struct {
	// Server configuration
	config *Config
	// Number of active clients
	activeClients uint
}

// A listener is listening on a given address. Ex: 0.0.0.0:193
type Listener struct {
	Addr string
}

// An IMAP Client as seen by an IMAP server
type client struct {
	conn   net.Conn
	bufin  *bufio.Reader
	bufout *bufio.Writer
	id     int
	config *Config
}

// Create an IMAP server
func Create(config *Config) *Server {
	server := new(Server)
	server.config = config

	return server
}

// Return the default server configuration
func DefaultConfig() *Config {
	listeners := []Listener{
		Listener{
			Addr: "0.0.0.0:143",
		},
	}

	return &Config{
		Listeners:  listeners,
		MaxClients: 8,
	}
}

// Add a mailstore to the config
func Store(m Mailstore) func(*Server) error {
	return func(s *Server) error {
		s.config.Mailstores = append(s.config.Mailstores, m)
		return nil
	}
}

// test if 2 listeners are equal
func equalListeners(l1, l2 []Listener) bool {
	for i, l := range l1 {
		if l != l2[i] {
			return false
		}
	}
	return true
}

// Add an interface to listen to
func Listen(Addr string) func(*Server) error {
	return func(s *Server) error {
		// if we only have the default config we should override it
		dc := DefaultConfig()
		l := Listener{
			Addr: Addr,
		}
		if equalListeners(dc.Listeners, s.config.Listeners) {
			s.config.Listeners = []Listener{l}
		} else {
			s.config.Listeners = append(s.config.Listeners, l)
		}

		return nil
	}
}

// Set MaxClients config
func MaxClients(max uint) func(*Server) error {
	return func(s *Server) error {
		s.config.MaxClients = max
		return nil
	}
}

func NewServer(options ...func(*Server) error) *Server {
	// set the default config
	s := &Server{}
	dc := DefaultConfig()
	s.config = dc

	// override the config with the functional options
	for _, option := range options {
		err := option(s)
		if err != nil {
			panic(err)
		}
	}

	//Check if we can listen on default ports, if not try to find a free port
	if equalListeners(dc.Listeners, s.config.Listeners) {
		listener := s.config.Listeners[0]
		l, err := net.Listen("tcp", listener.Addr)
		if err != nil {
			l, err = net.Listen("tcp4", ":0") // this will ask the OS to give us a free port
			if err != nil {
				panic("Can't listen on any port")
			}
			l.Close()
			s.config.Listeners[0].Addr = l.Addr().String()
		} else {
			l.Close()
		}
	}

	return s
}

// Start an IMAP server
func (s *Server) Start() error {
	// Start listening for IMAP connections
	for _, iface := range s.config.Listeners {
		listener, err := net.Listen("tcp", iface.Addr)
		if err != nil {
			log.Fatalf("IMAP cannot listen on %s, %v", iface.Addr, err)
		}

		log.Print("IMAP server listening on ", iface.Addr)

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
				id:     clientNumber,
				config: s.config,
			}

			go client.handle()

			clientNumber += 1
		}
	}
	return nil
}

// Handle requests from an IMAP client
func (c *client) handle() {

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
	sess := createSession(c.id, c.config)

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
	log.Printf("IMAP client %d, %v", c.id, err)
}
