package imapsrv

import "strings"

// capability is a CAPABILITY command
type capability struct {
	tag string
}

// createCapability creates a CAPABILITY command
func createCapability(p *parser, tag string) command {
	return &capability{tag: tag}
}

// execute a capability
func (c *capability) execute(s *session, out chan response) {
	defer close(out)
	var commands []string

	if s.st == notAuthenticated {
		switch s.listener.encryption {
		case unencryptedLevel:
		// TODO: do we want to support this?

		case starttlsLevel:
			if s.encryption == tlsLevel {
				// would be the case, if we actually supported it
				// commands = append(commands, "AUTH=PLAIN")
			} else {
				commands = append(commands, "STARTTLS")
				commands = append(commands, "LOGINDISABLED")
			}

		case tlsLevel:
			// would be the case, if we actually supported it
			// commands = append(commands, "AUTH=PLAIN")
		}
	} else {
		// Things that are supported after authenticating
		// commands = append(commands, "CHILDREN")
	}

	// Return all capabilities
	out <- ok(c.tag, "CAPABILITY completed").
		putLine("CAPABILITY IMAP4rev1 " + strings.Join(commands, " "))
}

func init() {
	registerCommand("capability", createCapability)
}
