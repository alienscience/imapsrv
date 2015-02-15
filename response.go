package imapsrv

import (
	"bufio"
)

// An IMAP response
type response interface {
	// Put a text line or lines into the response
	put(s string) response
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

// Add an untagged string to a final response
// TODO: have putLine and put
func (r *finalResponse) put(s string) response {
	r.partialResponse.put(s)
	return r
}

// Add an untagged string to a partial response
func (r *partialResponse) put(s string) response {
	r.entries = append(r.entries, s)
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
		_, err := w.WriteString("* " + line + "\r\n")
		if err != nil {
			return err
		}
	}

	return nil
}
