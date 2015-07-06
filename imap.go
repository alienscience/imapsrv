// An IMAP server
package imapsrv

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"github.com/alienscience/imapsrv/auth"
	"log"
	"net"
	"net/textproto"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// DefaultListener is the listener that is used if no listener is specified
const DefaultListener = "0.0.0.0:143"

// config is an IMAP server configuration
type Config struct {
	MaxClients uint
	Listeners  []listener
	Mailstore  Mailstore

	AuthBackend auth.AuthStore

	LmtpEndpoints []endPoint

	// Hostname is the hostname of this entire server
	Hostname string

	// Production indicates whether or not this is used in production
	// - disabling this allows for the program to panic
	Production bool
	
	// AliasMapEndpoints indicate the endpoints on which to advertise the available addresses
	// TODO: use / advertise these
	AliasMapEndpoints []endPoint
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
	config *Config
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
	config *Config
}

// defaultConfig returns the default server configuration
func defaultConfig() *Config {
	return &Config{
		Listeners:  make([]listener, 0, 4),
		MaxClients: 8,
	}
}

// Add a mailstore to the config
// StoreOption add a mailstore to the config
func StoreOption(m Mailstore) option {
	return func(s *Server) error {
		s.config.Mailstore = m
		return nil
	}
}

// AuthStoreOption adds an authenticaton backend
func AuthStoreOption(a auth.AuthStore) option {
	return func(s *Server) error {
		s.config.AuthBackend = a
		return nil
	}
}

// ListenOption adds an interface to listen to
func ListenOption(Addr string) option {
	return func(s *Server) error {
		l := listener{
			addr: Addr,
		}
		s.config.Listeners = append(s.config.Listeners, l)
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
		s.config.Listeners = append(s.config.Listeners, l)
		return nil
	}
}

// MaxClientsOption sets the MaxClients config
func MaxClientsOption(max uint) option {
	return func(s *Server) error {
		s.config.MaxClients = max
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
	if len(s.config.Listeners) == 0 {
		s.config.Listeners = append(s.config.Listeners,
			listener{addr: DefaultListener})
	}

	var err error
	// Start listening for IMAP connections
	for i, iface := range s.config.Listeners {
		s.config.Listeners[i].listener, err = net.Listen("tcp", iface.addr)
		if err != nil {
			log.Printf("IMAP cannot listen on %s, %v", iface.addr, err)
			return err
		}
		s.socketsMu.Lock()
		s.sockets = append(s.sockets, s.config.Listeners[i].listener)
		s.socketsMu.Unlock()
	}

	// Start the LMTP entrypoints as desired
	for i, entrypoint := range s.config.LmtpEndpoints {
		go s.runLMTPListener(entrypoint, i)
	}

	// Start the server on each port
	for i, listener := range s.config.Listeners {
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
			if err, ok := e.(parseError); ok {
				c.logError(err)
				fatalResponse(c.bufout, err)
			} else {
				if s.config.Production {
					log.Println("Panic:", e)
				} else {
					panic(e)
				}
			}
		}
	}()

	// Create a parser
	parser := createParser(c.bufin)

	// Write the welcome message
	err := ok("*", "IMAP4rev1 Service Ready").writeTo(c.bufout)

	if err != nil {
		c.logError(err)
		return
	}

	//  Create a session
	sess := createSession(c.id, c.config, s, &c.listener, c.conn)

	for {
		// Get the next IMAP command
		command := parser.next()

		// Execute the IMAP command and finish when requested
		if open, newBuffers := c.execute(command, sess); !open {
			break
		} else if newBuffers != nil {
			c.bufin = newBuffers.Reader.R
			c.bufout = newBuffers.Writer.W
			parser.lexer.reader.R = newBuffers.R
		}
	}
}

// Execute an IMAP command in the given session
// Returns true if execution can continue, false if not
func (c *imapClient) execute(cmd command, sess *session) (keepOpen bool, newBuffers *textproto.Conn) {

	// Create an output channel
	ch := make(chan response)

	// Execute the command in the background
	// TODO: (AlienScience) support concurrent command execution
	// TODO: (EtienneBruines) what would need to be fixed in order to support that?
	go cmd.execute(sess, ch)

	keepOpen = true

	// Output the responses
	for r := range ch {
		err := r.writeTo(c.bufout)

		if final, ok := r.(*finalResponse); ok {
			newBuffers = final.bufReplacement
		}

		if err != nil {
			c.logError(err)
			keepOpen = false
		}

		// Should the connection be closed?
		if r.isClose() {
			keepOpen = false
		}

	}

	return
}

// close closes an IMAP client
func (c *imapClient) close() {
	c.conn.Close()
}

// logError sends a log message to the default Logger
func (c *imapClient) logError(err error) {
	log.Printf("IMAP client %s, %v", c.id, err)
}
