package imapsrv

import (
	"crypto/tls"
	"fmt"
	"net/textproto"
)

type starttls struct {
	tag string
}

// starttls creates a starttls command
func createStarttls(p *parser, tag string) command {
	return &starttls{tag: tag}
}

func (c *starttls) execute(sess *session, out chan response) {
	defer close(out)

	sess.conn.Write([]byte(fmt.Sprintf("%s OK Begin TLS negotiation now\r\n", c.tag)))

	tlsConn := tls.Server(sess.conn, &tls.Config{Certificates: sess.listener.certificates})

	sess.conn = tlsConn
	textConn := textproto.NewConn(sess.conn)

	sess.encryption = tlsLevel
	out <- empty().shouldReplaceBuffers(textConn)
}

func init() {
	registerCommand("starttls", createStarttls)
}
