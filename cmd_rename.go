package imapsrv

import (
	"log"
	"strings"
)

// rename allows one to change the name of a mailbox
type rename struct {
	tag     string
	oldname string
	newname string
}

// createRename creates a RENAME command
func createRename(p *parser, tag string) command {
	// Get the mailbox name
	oldname := p.ExpectString(p.lexer.astring)
	newname := p.ExpectString(p.lexer.astring)

	return &rename{tag: tag, oldname: oldname, newname: newname}

}
func (c *rename) execute(sess *session, out chan response) {
	defer close(out)
	box, err := sess.config.Mailstore.Mailbox(sess.user, strings.Split(c.oldname, string(pathDelimiter)))
	if err != nil {
		out <- no(c.tag, "mailbox not found")
		return
	}

	// TODO: this works, but perhaps we should use a function .Exists() ?
	newBox, err := sess.config.Mailstore.Mailbox(sess.user, strings.Split(c.newname, string(pathDelimiter)))
	if newBox != nil {
		out <- no(c.tag, "name already taken")
		return
	}

	err = box.Rename(strings.Split(c.newname, string(pathDelimiter)))
	if err != nil {
		log.Println(err)
		out <- no(c.tag, "error occured while renaming")
		return
	}
	out <- ok(c.tag, "RENAME Completed")
}

func init() {
	registerCommand("rename", createRename)
}
