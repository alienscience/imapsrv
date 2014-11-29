package imapsrv

import (
	"bufio"
	"strings"
	"testing"
)

func TestQstring(t *testing.T) {

	r := bufio.NewReader(strings.NewReader("quoted string\"\n"))
	l := createLexer(r)
	l.skipSpace()
	tk := l.qstring()

	if tk.value != "quoted string" {
		t.Fail()
	}

}

func TestLiteral(t *testing.T) {

	r := bufio.NewReader(strings.NewReader("0}\n"))
	l := createLexer(r)
	l.skipSpace()
	tk := l.literal()

	if tk.value != "" {
		t.Fail()
	}

}

// TestAstring checks the lexer will return a valid <astring> per the ABNF rule, or panic on a failing test
//
// Astring = 1*ASTRING-CHAR / string
//     ASTRING-CHAR = ATOM-CHAR / resp-specials
//         ATOM-CHAR = <any CHAR except atom-specials>
//             atom-specials = "(" / ")" / "{" / SP / CTL / list-wildcards / quoted-specials / resp-specials
//                 list-wildcards = "%" / "*"
//                 quoted-specials = DQUOTE / "\"
//                 resp-specials   = "]"
//     string = quoted / literal
//         quoted = DQUOTE *QUOTED-CHAR DQUOTE
//             QUOTED-CHAR = <any TEXT-CHAR except quoted-specials> / "\" quoted-specials
//                 TEXT-CHAR = <any CHAR except CR and LF>
//                 quoted-specials = DQUOTE / "\"
//         literal = "{" number "}" CRLF *CHAR8 ; number represents the number of CHAR8s
//
// SP  = %x20
// CTL = %x00-1F / %x7F ; controls
// DQUOTE = %x22
// CR  = %x0D
// LF  = %x0A
func TestAstring(t *testing.T) {

	// Test cases receive a map of OUTPUT => INPUT
	passing := map[string]string{
		"a":     "a\r\n",     // 1*ASTRING-CHAR - single
		"this":  "this\r\n",  // 1*ASTRING-CHAR - many
		"burb":  "burb)\r\n", // 1*ASTRING-CHAR - stop at )
		"":      "\"\"\r\n",  // <quoted> with no *QUOTED-CHAR
		"[":     "[\r\n",
		" abcd": "{5}\r\n abcd\n", // <string> alternative <literal>
		"]":     "]\n",            // <ASTRING-CHAR> under the <resp-specials> alternative
	}

	// The failing test case map key is largely irrelevant as they should panic, just included for consistency
	failing := map[string]string{
		" ": " ", // SP
		//"":   "",   // 1*ASTRING-CHAR should have at least one char // TODO : Gets EOF -- should panic?
		"\\": "\\", // <quoted-specials> not allowed in ATOM-CHAR
		//"\"": "\"", // DQUOTE // TODO : Gets EOF -- should panic?
		"%": "%", // <list-wildcard>
		"*": "*", // <list-wildcard>
		")": ")", // <atom-specials> not allowed in ATOM-CHAR
		"(": "(", // <atom-specials> not allowed in ATOM-CHAR
	}

	panicCount := 0

	testAstring := func(in, out string) (bool, string) {

		// Catch the panics and increment the panic counter for failures
		defer func() {
			if r := recover(); r != nil {
				// EOFs are easily obscured as they are also a form of panic in the system
				// but do not constitute an 'expected' panic type here
				if r.(parseError).Error() == "EOF" {
					t.Logf("Bad panic on input: %q, output: %q", in, out)
					panic("EOF found in TestAstring - should not be present, correct the test(s)")
				}
				panicCount += 1
			}
		}()

		r := bufio.NewReader(strings.NewReader(in))
		l := createLexer(r)
		tk := l.next(asAString)

		return tk.value == out, tk.value

	}

	for o, i := range passing {
		ok, actual := testAstring(i, o)
		if !ok {
			t.Logf("Failed on passing case: input %q, expected output %q, actual output %q",
				i, o, actual)
			t.Fail()
		}
	}

	for o, i := range failing {
		ok, _ := testAstring(i, o)
		if ok {
			// This should not be reached as all failing test cases should trigger a panic
			t.Logf("Failed on failing case: input %q, output %q", i, o)
			t.Fail()
		}
	}

	if panicCount != len(failing) {
		t.Logf("Expected %d panics, found %d", len(failing), panicCount)
		t.Fail()
	}

}

func TestSkipSpace(t *testing.T) {

	r := bufio.NewReader(strings.NewReader("abc one"))
	l := createLexer(r)

	// lexer instantiates with space at current
	if l.current != byte(' ') {
		t.Fail()
	}

	l.skipSpace()
	// skips past the initialized space
	if l.current != byte('a') {
		t.Fail()
	}

}

func TestConsumeEol(t *testing.T) {

	r := bufio.NewReader(strings.NewReader("abc\none"))
	l := createLexer(r)
	l.consumeEol()

	if l.current != byte('\n') {
		t.Fail()
	}

}

func TestConsume(t *testing.T) {

	r := bufio.NewReader(strings.NewReader("abc\none"))
	l := createLexer(r)
	l.skipSpace()
	l.consume()

	if l.current != byte('b') {
		t.Fail()
	}

}

func TestLexesAstring(t *testing.T) {

	r := bufio.NewReader(strings.NewReader("a0001)\n"))
	l := createLexer(r)
	token := l.next(asAString)

	if token.tokType != stringTokenType {
		t.Fail()
	}

	if token.value != "a0001" {
		t.Fail()
	}

}

func TestLexesQuotedString(t *testing.T) {

	r := bufio.NewReader(strings.NewReader("\"A12312\"\n"))
	l := createLexer(r)
	token := l.next(asAString)

	if token.tokType != stringTokenType {
		t.Fail()
	}

	if token.value != "A12312" {
		t.Fail()
	}

}

func TestLexesLiteral(t *testing.T) {

	r := bufio.NewReader(strings.NewReader("{11}\nFRED FOOBAR {7}\n"))
	l := createLexer(r)
	token := l.next(asAString)

	if token.tokType != stringTokenType {
		t.Fail()
	}

	// the token after {11} should be of length 11
	if 11 != len(token.value) {
		t.Fail()
	}

	if "FRED FOOBAR" != token.value {
		t.Fail()
	}

}
