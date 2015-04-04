package imapsrv

import (
	"bufio"
	"bytes"
	"fmt"
)

// Anything longer than this is considered long and MIGHT be split
// into a literal
const longLineLength = 80

// An IMAP response
type response interface {
	// Put a string into the response (no newline)
	put(s string) response
	// Put a text line or lines into the response
	putLine(s string) response
	// Put a field into the response (no newline)
	putField(name string, value string) response
	// Output the response
	writeTo(w *bufio.Writer) error
	// Should the connection be closed?
	isClose() bool
}

// A final response that is sent when a command completes
type finalResponse struct {
	// The tag of the command that this is the response for
	tag string
	// The machine readable condition
	condition string
	// A human readable message
	message string
	// Should the connection be closed after the response has been sent?
	closeConnection bool
	// Untagged output
	partialResponse
}

// An partial response that can be sent before a command completes
type partialResponse struct {
	// The current entry being built
	current *bytes.Buffer
	// Was the last call a putField?
	fields bool
	// The previous entries
	entries []string
}

// Create a final response
func createFinalResponse(tag string, condition string, message string) *finalResponse {
	return &finalResponse{
		tag:             tag,
		condition:       condition,
		message:         message,
		partialResponse: createPartialResponse(),
	}
}

// Create a partial response
func createPartialResponse() partialResponse {
	return partialResponse{
		entries: make([]string, 0, 4),
	}
}

// Create a pointer to a partial response
func partial() *partialResponse {
	ret := createPartialResponse()
	return &ret
}

// Create a OK response
func ok(tag string, message string) *finalResponse {
	return createFinalResponse(tag, "OK", message)
}

// Create an BAD response
func bad(tag string, message string) *finalResponse {
	return createFinalResponse(tag, "BAD", message)
}

// Create a NO response
func no(tag string, message string) *finalResponse {
	return createFinalResponse(tag, "NO", message)
}

// Create an untagged fatal response
func fatalResponse(w *bufio.Writer, err error) *finalResponse {
	resp := createFinalResponse("*", "BYE", err.Error())
	resp.closeConnection = true
	return resp
}

// Add a string to a final response
func (r *finalResponse) put(s string) response {
	r.partialResponse.put(s)
	return r
}

// Add an untagged string to a final response
func (r *finalResponse) putLine(s string) response {
	r.partialResponse.putLine(s)
	return r
}

// Add a field to a final response
func (r *finalResponse) putField(name string, value string) response {
	r.partialResponse.putField(name, value)
	return r
}

// Add a string to a partial response
func (r *partialResponse) put(s string) response {

	// Add the string to the current entry
	if r.current.Len() == 0 {
		r.current = bytes.NewBufferString(s)
	} else {
		r.current.WriteString(s)
	}

	r.fields = false
	return r
}

// Add an untagged line to a partial response
func (r *partialResponse) putLine(s string) response {
	if r.current.Len() > 0 {
		r.entries = append(r.entries, r.current.String())
	}
	r.current = bytes.NewBufferString(s)
	r.fields = false
	return r
}

// Add a field to a partial response
func (r *partialResponse) putField(name string, value string) response {

	// Add the field name to the current entry
	if r.current.Len() == 0 {
		r.current = bytes.NewBufferString(name)
	} else {
		// Fields are space separated if written together
		if r.fields {
			r.current.WriteString(" ")
		}
		r.current.WriteString(name)
	}

	// Is this a long field value?
	if len(value) > longLineLength {
		appendLiteral(r.current, value)
	} else {
		r.current.WriteString(" ")
		r.current.WriteString(value)
	}

	r.fields = true
	return r
}

// Mark that a response should close the connection
func (r *finalResponse) shouldClose() *finalResponse {
	r.closeConnection = true
	return r
}

// Should a final response close the connection?
func (r *finalResponse) isClose() bool {
	return r.closeConnection
}

// Should a partial response close the connection?
func (r *partialResponse) isClose() bool {
	return false
}

// Write a final response to the given writer
func (r *finalResponse) writeTo(w *bufio.Writer) error {

	// Write untagged lines
	err := r.partialResponse.writeTo(w)

	if err != nil {
		return err
	}

	_, err = w.WriteString(r.tag + " " + r.condition + " " + r.message + "\r\n")

	if err != nil {
		return err
	}

	// Flush the response
	w.Flush()
	return nil
}

// Write a partial response to the given writer
func (r *partialResponse) writeTo(w *bufio.Writer) error {

	// Write untagged lines
	for _, line := range r.entries {
		err := writeLine(w, line)
		if err != nil {
			return err
		}
	}

	// Write last line
	if r.current.Len() > 0 {
		err := writeLine(w, r.current.String())
		if err != nil {
			return err
		}
	}

	return nil
}

//---- Helper functions --------------------------------------------------------

// Append a string to a buffer as a literal
func appendLiteral(b *bytes.Buffer, s string) {

	// Append the string length
	b.WriteString(fmt.Sprint("{", len(s), "}\r\n"))

	// Append the literal
	b.WriteString(s)

	// Append the cr/nl
	b.WriteString("\r\n")
}

// Write a line of partial response
func writeLine(w *bufio.Writer, s string) error {

	_, err := w.WriteString("* ")
	if err != nil {
		return err
	}
	_, err = w.WriteString(s)
	if err != nil {
		return err
	}
	_, err = w.WriteString("\r\n")
	if err != nil {
		return err
	}

	return nil
}
