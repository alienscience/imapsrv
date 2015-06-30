package imapsrv

import (
	"errors"
	"fmt"
	"github.com/jhillyerd/go.enmime"
	"log"
	"mime"
	"strings"
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

// TODO: see if the definitions below can be converted to functions
type partSpecifier int

const (
	invalidPart = iota
	headerPart
	headerFieldsPart
	headerFieldsNotPart
	textPart
	mimePart
)

// A byte range
type fetchPartial struct {
	fromOctet int32
	toOctet   uint32
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
	return nil
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
			/* TODO: fix and uncomment (it may not work, but at least now it compiles)
			case bodyTypeMessage:
				envelope := getEnvelope(header)
				return strings.Join([]string{mediaType, bodyFields, envelope},
					" "), nil
			case bodyTypeText:
				lineCount := fmt.Sprint(countLines(msg.Body))
				return strings.Join([]string{mediaType, bodyFields, lineCount},
					" "), nil
			*/
		default:
			return "", errors.New("Unknown body type")
		}
	} else {

		// body-type-mpart
		mime, err := wrap.getMime()
		if err != nil {
			return "", err
		}

		root := mime.Root
		log.Println("Debug:", root)
		// TODO: finish this
		return "", nil
	}
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

//---- UID ---------------------------------------------------------------------

type uidFetchAtt struct{}

func (a *uidFetchAtt) extract(resp response, msg *messageWrap) error {
	return nil
}

//---- BODY.PEEK ---------------------------------------------------------------

type bodyPeekFetchAtt struct {
	fetchSection
}

func (a *bodyPeekFetchAtt) extract(resp response, msg *messageWrap) error {
	return nil
}
