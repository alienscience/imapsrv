package imapsrv

import (
	"fmt"
	"log"
	"strings"
)

type lsub struct {
	tag     string
	refname string
	mailbox string
}

func createLsub(p *parser, tag string) command {
	refname := p.expectString(p.lexer.astring)
	mailbox := p.expectString(p.lexer.astring)
	return &lsub{tag, refname, mailbox}
}

func (c *lsub) execute(sess *session, out chan response) {
	defer close(out)

	// Convert the reference and mbox pattern into slices
	ref := pathToSlice(c.refname)
	mbox := pathToSlice(c.mailbox)

	// Get the list of mailboxes
	mboxes, err := sess.list(ref, mbox)
	if err != nil {
		log.Println("LSUB failed;", err)
		out <- no(c.tag, "LSUB failed; could not fetch list")
		return
	}

	// Respond with the mailboxes
	res := ok(c.tag, "LSUB completed")
	for _, mbox := range mboxes {
		sub, err := mbox.provider.Subscribed()
		if err != nil {
			log.Println("Error checking if subscribed", err)
			continue
		}
		if sub {
			res.putLine(fmt.Sprintf(`LSUB (%s) "%s" "%s"`,
				joinMailboxFlags(mbox),
				string(pathDelimiter),
				strings.Join(mbox.provider.Path(), string(pathDelimiter))))
		} else {
			des, err := mbox.provider.SubscribedDescendant()
			if err != nil {
				log.Println("Error while checking descendants:", err)
				continue
			}
			if des {
				res.putLine(fmt.Sprintf(`LSUB (%s) "%s" "%s"`,
					"\\Noselect",
					string(pathDelimiter),
					strings.Join(mbox.provider.Path(), string(pathDelimiter))))
			}
		}
	}

	out <- res
}

func init() {
	registerCommand("lsub", createLsub)
}
