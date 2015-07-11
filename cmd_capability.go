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

	caps := capabilities[s.st][s.encryption]

	// Return all capabilities
	out <- ok(c.tag, "CAPABILITY completed").
		putLine("CAPABILITY IMAP4rev1 " + strings.Join(caps, " "))
}

var capabilities = [][][]string{
	notAuthenticated: [][]string{
		unencryptedLevel: []string{},
		starttlsLevel:    []string{},
		tlsLevel:         []string{},
	},
	authenticated: [][]string{
		unencryptedLevel: []string{},
		starttlsLevel:    []string{},
		tlsLevel:         []string{},
	},
	selected: [][]string{
		unencryptedLevel: []string{},
		starttlsLevel:    []string{},
		tlsLevel:         []string{},
	},
}

func registerCapability(cap string, s state, e encryptionLevel) {
	capabilities[s][e] = append(capabilities[s][e], strings.ToUpper(cap))
}

func init() {
	registerCommand("capability", createCapability)
}
