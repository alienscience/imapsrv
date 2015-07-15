package imapsrv

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"mime"
	"net/mail"
	"strconv"
	"strings"

	enmime "github.com/jhillyerd/go.enmime"
)

// A fetch attachment extracts part of a message and adds it to the response.
// Fetch attachment structs are created by the lexer and parser and then
// executed by the session.
type fetchAttachment interface {
	extract(resp response, msg *messageWrap) error
}

// Some fetch attachments have to extract from sections and byte ranges
type fetchSection struct {
	section []uint32
	part    partSpecifier
	fields  []string
	partial *fetchPartial // nil if no fetchPartial exists
}

// Part specifiers
// TODO: see if the definitions below can be converted to functions
type partSpecifier int

const (
	noPartSpecifier = iota
	headerPart
	headerFieldsPart
	headerFieldsNotPart
	textPart
	mimePart
)

// A byte range
type fetchPartial struct {
	fromOctet uint32
	length    uint32
}

//---- fetchSection ------------------------------------------------------------

// The text description of a section specification
func (s *fetchSection) spec() string {

	ret := ""

	// Section identifier
	i := 0
	for ; i < len(s.section); i += 1 {
		if i > 0 {
			ret += "."
		}
		ret += strconv.FormatUint(uint64(s.section[i]), 10)
	}

	// Are there part specifiers?
	if s.part == noPartSpecifier {
		return ret
	}

	// Add the part specifier
	if i > 0 {
		ret += "."
	}

	switch s.part {
	case headerPart:
		ret += "HEADER"
	case headerFieldsPart:
		ret += "HEADER.FIELDS "
		ret += s.fieldsSpec()
	case headerFieldsNotPart:
		ret += "HEADER.FIELDS.NOT "
		ret += s.fieldsSpec()
	case textPart:
		ret += "TEXT"
	case mimePart:
		ret += "MIME"
	default:
		// Add nothing
	}

	return ret
}

// Section specifier header fields as a string
func (s *fetchSection) fieldsSpec() string {

	ret := "("
	for i := 0; i < len(s.fields); i += 1 {
		if i > 0 {
			ret += " "
		}
		ret += s.fields[i]
	}
	ret += ")"
	return ret
}

//---- ENVELOPE ----------------------------------------------------------------

type envelopeFetchAtt struct{}

func (a *envelopeFetchAtt) extract(resp response, msg *messageWrap) error {
	mime, err := msg.getMime()
	if err != nil {
		return err
	}

	// Add header fields
	root := mime.Root
	if root == nil {
		// TODO: what should we do in this case?
		return fmt.Errorf("mime root could not be determined")
	}
	header := root.Header()
	env := fmt.Sprint(
		"(",
		header["Date"], " ",
		header["Subject"], " ",
		header["From"], " ",
		header["Sender"], " ",
		header["Reply-To"], " ",
		header["To"], " ",
		header["Cc"], " ",
		header["Bcc"], " ",
		header["Bcc"], " ",
		header["In-Reply-To"], " ",
		header["Message-ID"],
		")")
	resp.putField("ENVELOPE", env)

	return nil
}

//---- FLAGS -------------------------------------------------------------------

type flagsFetchAtt struct{}

func (a *flagsFetchAtt) extract(resp response, msg *messageWrap) error {

	flags, err := msg.provider.Flags()
	if err != nil {
		return err
	}

	// Convert flags to strings
	resp.putField("FLAGS", "("+joinMessageFlags(flags)+")")
	return nil
}

// Return a string of message flags
func joinMessageFlags(flags uint8) string {

	// Convert the mailbox flags into a slice of strings
	ret := make([]string, 0, 4)

	for flag, str := range messageFlags {
		if flags&flag != 0 {
			ret = append(ret, str)
		}
	}

	// Return a joined string
	return strings.Join(ret, " ")
}

//---- RFC822.HEADER -----------------------------------------------------------

type rfc822HeaderFetchAtt struct{}

func (a *rfc822HeaderFetchAtt) extract(resp response, msg *messageWrap) error {

	// Get the raw header
	hdr, err := msg.rfc822Header()
	if err != nil {
		return err
	}
	resp.putField("RFC822.HEADER", hdr)

	return nil
}

//---- INTERNALDATE ------------------------------------------------------------

type internalDateFetchAtt struct{}

func (a *internalDateFetchAtt) extract(resp response, msg *messageWrap) error {
	date, err := msg.internalDate()
	if err != nil {
		return err
	}
	resp.putField("INTERNALDATE", date)

	return nil
}

//---- RFC822.SIZE -------------------------------------------------------------

type rfc822SizeFetchAtt struct{}

func (a *rfc822SizeFetchAtt) extract(resp response, msg *messageWrap) error {

	size, err := msg.size()
	if err != nil {
		return err
	}
	resp.putField("RFC822.SIZE", fmt.Sprint(size))

	return nil
}

//---- RFC822.TEXT -------------------------------------------------------------

type rfc822TextFetchAtt struct{}

func (a *rfc822TextFetchAtt) extract(resp response, msg *messageWrap) error {

	mime, err := msg.getMime()
	if err != nil {
		return err
	}

	// Like BODY[TEXT]
	resp.putField("RFC822.TEXT", mime.Text)

	return nil
}

//---- BODY --------------------------------------------------------------------

// Body without a section
type bodyFetchAtt struct{}

func (a *bodyFetchAtt) extract(resp response, msg *messageWrap) error {

	// Like BODYSTRUCTURE without extensions
	structure, err := bodyStructure(msg, false)
	if err != nil {
		return err
	}

	resp.putField("BODY", structure)

	return nil
}

// Body with a section
type bodySectionFetchAtt struct {
	fetchSection
}

func (a *bodySectionFetchAtt) extract(resp response, msg *messageWrap) error {
	mime, err := msg.getMime()
	if err != nil {
		return err
	}

	currentSection := mime.Root
	fs := a.fetchSection

	// Go to the requested section
	for i := 0; i < len(fs.section); i++ {
		subsection := fs.section[i]

		var j uint32
		for j = 1; j < subsection; j++ {
			// Move across
			currentSection = currentSection.NextSibling()
		}
		if i < len(fs.section)-1 {
			// Move down
			currentSection = currentSection.FirstChild()
		}
	}

	// Consider the part specifier
	var payload string

	switch fs.part {
	case noPartSpecifier:
		payload = extractPartial(currentSection, fs.partial)
	case headerPart:
		// TODO
		//payload = extractHeader(currentSection)
	case headerFieldsPart:
		// TODO
		//payload = extractHeaderFields(currentSection, fs.fields)
	case headerFieldsNotPart:
		// TODO
		//payload = extractHeaderNotFields(currentSection, fs.fields)
	case textPart:
		// TODO
		//payload = string(currentSection.Content())
	case mimePart:
		// TODO: check if MIME-IMB and MIME-IMT headers are the same
		payload = extractHeader(currentSection)
	}

	// Add the section information to the field name
	sectionSpec := fs.spec()
	fieldName := fmt.Sprint("BODY", sectionSpec)
	resp.putField(fieldName, payload)

	return nil
}

func extractPartial(section enmime.MIMEPart, partial *fetchPartial) string {

	content := section.Content()

	if partial == nil {
		// Return the whole section
		return string(content)
	}

	// If this point is reached, return part of the content
	highIndex := partial.fromOctet + partial.length
	partialContent := content[partial.fromOctet:highIndex]
	return string(partialContent)
}

func extractHeader(section enmime.MIMEPart) string {

	headerMap := section.Header()

	// Rebuild the header string from the parsed datastructure
	ret := ""
	for header, vList := range headerMap {
		for _, value := range vList {
			ret += header + ": " + value
		}
	}

	return ret
}

func extractHeaderFields(section enmime.MIMEPart, fields []string) string {

	headerMap := section.Header()

	// Partially rebuild the header string from the parsed
	// datastructure using the given fields
	ret := ""
	for _, header := range fields {
		vList := headerMap[header]
		for _, value := range vList {
			ret += header + ": " + value
		}
	}

	return ret
}

func extractHeaderNotFields(section enmime.MIMEPart, fields []string) string {

	headerMap := section.Header()

	// Build a set of fields to exclude
	excludeField := make(map[string]bool)
	for _, field := range fields {
		excludeField[field] = true
	}

	// Partially rebuild the header string from the parsed
	// datastructure excluding the given fields
	ret := ""
	for header, vList := range headerMap {
		if !excludeField[header] {
			for _, value := range vList {
				ret += header + ": " + value
			}
		}
	}

	return ret
}

//---- BODYSTRUCTURE -----------------------------------------------------------

type bodyStructureFetchAtt struct{}

const (
	bodyTypeUnknown = iota
	bodyTypeBasic
	bodyTypeMessage
	bodyTypeText
)

func (a *bodyStructureFetchAtt) extract(resp response, msg *messageWrap) error {

	// Include extensions
	structure, err := bodyStructure(msg, true)
	if err != nil {
		return err
	}

	resp.putField("BODYSTRUCTURE", structure)

	return nil
}

// body            = "(" (body-type-1part / body-type-mpart) ")"
// body-type-1part = (body-type-basic / body-type-msg / body-type-text)
//                   [SP body-ext-1part]
//
// body-type-basic = media-basic SP body-fields
//                   ; MESSAGE subtype MUST NOT be "RFC822"
//
// body-type-mpart = 1*body SP media-subtype
//                   [SP body-ext-mpart]
//
// body-type-msg   = media-message SP body-fields SP envelope
//                   SP body SP body-fld-lines
//
// body-type-text  = media-text SP body-fields SP body-fld-lines
// body-ext-1part  = body-fld-md5 [SP body-fld-dsp [SP body-fld-lang
//                   [SP body-fld-loc *(SP body-extension)]]]
//                     ; MUST NOT be returned on non-extensible
//                     ; "BODY" fetch
// body-ext-mpart  = body-fld-param [SP body-fld-dsp [SP body-fld-lang
//                   [SP body-fld-loc *(SP body-extension)]]]
//                    ; MUST NOT be returned on non-extensible
//                    ; "BODY" fetch
// body-fields     = body-fld-param SP body-fld-id SP body-fld-desc SP
//                   body-fld-enc SP body-fld-octets
func bodyStructure(wrap *messageWrap, ext bool) (string, error) {

	// Extract a mail.Message
	msg, err := wrap.getMessage()
	if err != nil {
		return "", err
	}

	header := msg.Header

	// Is this a multipart message?
	if !enmime.IsMultipartMessage(msg) {

		// body-type-1part
		contentType := header["Content-Type"]
		bodyType, mediaType, bodyParams := getMediaType(contentType)
		bodyFields, err := getBodyFields(wrap, bodyParams)
		if err != nil {
			return "", err
		}

		switch bodyType {
		case bodyTypeBasic:
			return strings.Join([]string{mediaType, bodyFields},
				" "), nil
		// TODO: fix and uncomment (it may not work, but at least now it compiles)
		case bodyTypeMessage:
			envelope := getEnvelope(header)
			return strings.Join([]string{mediaType, bodyFields, envelope},
				" "), nil

		case bodyTypeText:
			lineCount := fmt.Sprint(countLines(msg.Body))
			return strings.Join([]string{mediaType, bodyFields, lineCount},
				" "), nil
		default:
			return "", fmt.Errorf("Unknown body type: %v", bodyType)
		}
	} else {

		// body-type-mpart
		mime, err := wrap.getMime()
		if err != nil {
			return "", err
		}

		root := mime.Root
		log.Println("Debug:", root)
		// TODO: find out how to generate body-type-mpart and finish this
		return "", nil
	}
}

/* getEnvelope returns the Envelope of a certain message:
   A parenthesized list that describes the envelope structure of a
   message.  This is computed by the server by parsing the
   [RFC-2822] header into the component parts, defaulting various
   fields as necessary.

   The fields of the envelope structure are in the following
   order: date, subject, from, sender, reply-to, to, cc, bcc,
   in-reply-to, and message-id.  The date, subject, in-reply-to,
   and message-id fields are strings.  The from, sender, reply-to,
   to, cc, and bcc fields are parenthesized lists of address
   structures.

   An address structure is a parenthesized list that describes an
   electronic mail address.  The fields of an address structure
   are in the following order: personal name, [SMTP]
   at-domain-list (source route), mailbox name, and host name.

   [RFC-2822] group syntax is indicated by a special form of
   address structure in which the host name field is NIL.  If the
   mailbox name field is also NIL, this is an end of group marker
   (semi-colon in RFC 822 syntax).  If the mailbox name field is
   non-NIL, this is a start of group marker, and the mailbox name
   field holds the group name phrase.

   If the Date, Subject, In-Reply-To, and Message-ID header lines
   are absent in the [RFC-2822] header, the corresponding member
   of the envelope is NIL; if these header lines are present but
   empty the corresponding member of the envelope is the empty
   string.

   Note: some servers may return a NIL envelope member in the
		"present but empty" case.  Clients SHOULD treat NIL and
		empty string as identical.

		Note: [RFC-2822] requires that all messages have a valid
		Date header.  Therefore, the date member in the envelope can
		not be NIL or the empty string.

		Note: [RFC-2822] requires that the In-Reply-To and
		Message-ID headers, if present, have non-empty content.
		Therefore, the in-reply-to and message-id members in the
		envelope can not be the empty string.

	 If the From, To, cc, and bcc header lines are absent in the
	 [RFC-2822] header, or are present but empty, the corresponding
	 member of the envelope is NIL.

	 If the Sender or Reply-To lines are absent in the [RFC-2822]
	 header, or are present but empty, the server sets the
	 corresponding member of the envelope to be the same value as
	 the from member (the client is not expected to know to do
	 this).

		Note: [RFC-2822] requires that all messages have a valid
		From header.  Therefore, the from, sender, and reply-to
		members in the envelope can not be NIL.
*/
func getEnvelope(header mail.Header) string {
	var buf bytes.Buffer
	buf.WriteRune('(')

	// Date
	if date := header.Get("Date"); len(date) > 0 {
		buf.WriteRune('"')
		buf.WriteString(header.Get("Date"))
		buf.WriteString("\" ")
	} else {
		buf.WriteString("NIL ")
	}

	// Subject
	// TODO: should we escape Subject for "" ?
	if subj := header.Get("Subject"); len(subj) > 0 {
		buf.WriteRune('"')
		buf.WriteString(subj)
		buf.WriteString("\" ")
	} else {
		buf.WriteString("NIL ")
	}

	// From
	buf.WriteRune('"')
	buf.WriteString(header.Get("From")) // TODO: verify? WHat to do if this is empty?
	// TODO: "From" is often not NAME SP AT-DOMAIN-LIST SP MAILNAME SP HOSTNAME... how to fix?
	buf.WriteString("\" ")

	// Sender
	buf.WriteRune('"')
	if sender := header.Get("Sender"); len(sender) > 0 {
		buf.WriteString(sender)
	} else {
		buf.WriteString(header.Get("From"))
	}
	buf.WriteString("\" ")

	// Reply-To
	buf.WriteRune('"')
	if replyto := header.Get("Reply-To"); len(replyto) > 0 {
		buf.WriteString(replyto)
	} else {
		buf.WriteString(header.Get("From"))
	}
	buf.WriteString("\" ")

	// To
	if to := header.Get("To"); len(to) > 0 {
		buf.WriteRune('"')
		buf.WriteString(to)
		buf.WriteString("\" ")
	} else {
		buf.WriteString("NIL ")
	}

	// CC
	if cc := header.Get("CC"); len(cc) > 0 {
		buf.WriteRune('"')
		buf.WriteString(cc)
		buf.WriteString("\" ")
	} else {
		buf.WriteString("NIL ")
	}

	// BCC
	if bcc := header.Get("BCC"); len(bcc) > 0 {
		buf.WriteRune('"')
		buf.WriteString(bcc)
		buf.WriteString("\" ")
	} else {
		buf.WriteString("NIL ")
	}

	// In-reply to
	if replyto := header.Get("In-Reply-To"); len(replyto) > 0 {
		buf.WriteRune('"')
		buf.WriteString(replyto)
		buf.WriteString("\" ")
	} else {
		buf.WriteString("NIL ")
	}

	// Message-id
	buf.WriteRune('"')
	buf.WriteString(header.Get("Message-Id")) // TODO: verify? What to do if this is empty?
	buf.WriteString("\" ")

	buf.WriteRune(')')
	return buf.String()
}

// Returns a bodyType identifier, a media type string and media type parameters
func getMediaType(contentType []string) (uint8, string, map[string]string) {

	if len(contentType) == 0 {
		// There is no media type
		return bodyTypeMessage, `"MESSAGE" "RFC822"`, nil
	}

	mediaString, params, err := mime.ParseMediaType(contentType[0])
	if err != nil {
		// The media type is invalid
		return bodyTypeMessage, `"MESSAGE" "RFC822"`, nil
	}

	mediaTypes := strings.Split(mediaString, "/")

	// Get the body type id
	bodyType := uint8(bodyTypeUnknown)
	mediaType := strings.ToUpper(mediaTypes[0])

	switch mediaType {
	case "TEXT":
		bodyType = bodyTypeText
	default:
		bodyType = bodyTypeBasic

	}

	// Build the return strings
	mediaRet := fmt.Sprintf(`"%s" "%s"`, mediaType, strings.ToUpper(mediaTypes[1]))
	return bodyType, mediaRet, params
}

// body-fields     = body-fld-param SP body-fld-id SP body-fld-desc SP
//                   body-fld-enc SP body-fld-octets
// body-fld-desc   = nstring
// body-fld-enc    = (DQUOTE ("7BIT" / "8BIT" / "BINARY" / "BASE64"/
//                  "QUOTED-PRINTABLE") DQUOTE) / string
// body-fld-id     = nstring
// body-fld-octets = number
// body-fld-param  = "(" string SP string *(SP string SP string) ")" / nil
func getBodyFields(wrap *messageWrap, bodyParams map[string]string) (string, error) {

	msg, err := wrap.getMessage()
	if err != nil {
		return "", err
	}
	header := msg.Header

	// body-fld-param
	bodyFieldParam := "NIL"
	if len(bodyParams) > 0 {
		params := make([]string, 0, 4)
		for k, v := range bodyParams {
			kv := fmt.Sprintf(`"%s" "%s"`, k, v)
			params = append(params, kv)
		}
		bodyFieldParam = fmt.Sprint("(", strings.Join(params, " "), ")")
	}

	// body-fld-id
	messageIds := header["Message-ID"]
	bodyFieldId := "NIL"
	if len(messageIds) > 0 {
		bodyFieldId = fmt.Sprintf(`"%s"`, messageIds[0])
	}

	// body-fld-desc
	descs := header["Content-Description"]
	bodyFieldDesc := "NIL"
	if len(descs) > 0 {
		bodyFieldDesc = fmt.Sprintf(`"%s"`, descs[0])
	}

	// body-fld-enc
	encodings := header["Content-Transfer-Encoding"]
	bodyFieldEnc := `"7BIT"`
	if len(encodings) > 0 {
		//bodyFieldEnc := fmt.Sprintf(`"%s"`, strings.ToUpper(encodings[0]))
	}

	// body-field-octets
	octets, err := wrap.provider.Size()
	if err != nil {
		return "", err
	}
	bodyFieldOctets := fmt.Sprintf("%d", octets)

	return fmt.Sprint(
		bodyFieldParam, " ",
		bodyFieldId, " ",
		bodyFieldDesc, " ",
		bodyFieldEnc, " ",
		bodyFieldOctets), nil
}

// Count the lines produced by the given reader
func countLines(r io.Reader) int {
	scanner := bufio.NewScanner(r)
	lines := 0
	for scanner.Scan() {
		lines += 1
	}
	return lines
}

//---- UID ---------------------------------------------------------------------

type uidFetchAtt struct{}

func (a *uidFetchAtt) extract(resp response, msg *messageWrap) error {

	resp.putField("UID", fmt.Sprint(msg.uid))
	return nil
}

//---- BODY.PEEK ---------------------------------------------------------------

type bodyPeekFetchAtt struct {
	fetchSection
}

func (a *bodyPeekFetchAtt) extract(resp response, msg *messageWrap) error {
	// TODO
	// An alternate form of BODY[<section>] that does not implicitly
	//         set the \Seen flag.
	return nil
}
