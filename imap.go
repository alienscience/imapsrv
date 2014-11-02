
// An IMAP server
package imapsrv

import (
	"bufio"
	"log"
	"net"
)

// IMAP server configuration
type Config struct {
	Interface  string
	MaxClients int
	Store      Mailstore
}

// An IMAP Server
type Server struct {
	// Server configuration
	config *Config
	// Number of active clients
	activeClients int
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
	return &Config{
		Interface:  "0.0.0.0:193",
		MaxClients: 8,
	}
}

// Start an IMAP server
func (s *Server) Start() {

	// Start listening for IMAP connections
	iface := s.config.Interface
	listener, err := net.Listen("tcp", iface)
	if err != nil {
		log.Fatalf("IMAP cannot listen on %s, %v", iface, err)
	}

	log.Print("IMAP server listening on ", iface)

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
