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

func TestAstring(t *testing.T) {

	r := bufio.NewReader(strings.NewReader("tHiS_IS#A_VAL!D_ASTRING \n"))
	l := createLexer(r)
	l.skipSpace()
	tk := l.astring()

	if tk.value != "tHiS_IS#A_VAL!D_ASTRING" {
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

	if "\nFRED FOOBA" != token.value {
		t.Fail()
	}

}
