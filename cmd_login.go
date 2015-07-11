package imapsrv

import "log"

// login is a LOGIN command
type login struct {
	tag      string
	userId   string
	password string
}

// createLogin creates a LOGIN command
func createLogin(p *parser, tag string) command {
	// Get the command arguments
	userId := p.expectString(p.lexer.astring)
	password := p.expectString(p.lexer.astring)

	// Create the command
	return &login{tag: tag, userId: userId, password: password}
}

// Login command
func (c *login) execute(sess *session, out chan response) {
	defer close(out)

	// Has the user already logged in?
	if sess.st != notAuthenticated {
		message := "LOGIN already logged in"
		sess.log(message)
		out <- bad(c.tag, message)
		return
	}

	auth, err := sess.server.config.AuthBackend.Authenticate(c.userId, c.password)
	if auth {
		sess.st = authenticated
		sess.user = c.userId
		out <- ok(c.tag, "LOGIN completed")
		return
	}
	log.Println("Login request:", auth, err)

	// Fail by default
	out <- no(c.tag, "LOGIN failure")
}

func init() {
	registerCommand("login", createLogin)
}
