package imapsrv

import (
	"bufio"
)

// response represents an IMAP response
type response struct {
	// The tag of the command that this is the response for
	tag string
	// The machine readable condition
	condition string
	// A human readable message
	message string
	// Untagged output lines
	untagged []string
	// Should the connection be closed after the response has been sent?
	closeConnection bool
	// bufInReplacement is not-null if all incoming traffic should be read from this instead
	bufInReplacement *bufio.Reader
	// bufOutReplacement is not-null if all outgoing traffic should be written to this instead
	bufOutReplacement *bufio.Writer
}

// createResponse creates a response
func createResponse(tag string, condition string, message string) *response {
	return &response{
		tag:       tag,
		condition: condition,
		message:   message,
		untagged:  make([]string, 0, 4),
	}
}

// ok creatse a OK response
func ok(tag string, message string) *response {
	return createResponse(tag, "OK", message)
}

// bad creates a BAD response
func bad(tag string, message string) *response {
	return createResponse(tag, "BAD", message)
}

// no creates a NO response
func no(tag string, message string) *response {
	return createResponse(tag, "NO", message)
}

// empty creates an empty response
func empty() *response {
	return &response{}
}

// fatalResponse writes an untagged fatal response (BYE)
func fatalResponse(w *bufio.Writer, err error) {
	resp := createResponse("*", "BYE", err.Error())
	resp.closeConnection = true
	resp.write(w)
}

// extra adds an untagged line to a response
func (r *response) extra(line string) *response {
	r.untagged = append(r.untagged, line)
	return r
}

// shouldClose marks that a response should close the connection
func (r *response) shouldClose() *response {
	r.closeConnection = true
	return r
}

// replaceBuffers sets two possible buffers that need replacement
func (r *response) replaceBuffers(reader *bufio.Reader, writer *bufio.Writer) *response {
	r.bufInReplacement = reader
	r.bufOutReplacement = writer
	return r
}

// write will write a response to the given writer
func (r *response) write(w *bufio.Writer) error {

	// Write untagged lines
	for _, line := range r.untagged {
		_, err := w.WriteString("* " + line + "\r\n")
		if err != nil {
			return err
		}
	}

	_, err := w.WriteString(r.tag + " " + r.condition + " " + r.message + "\r\n")
	if err != nil {
		return err
	}

	// Flush the response
	w.Flush()
	return nil
}
