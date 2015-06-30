package boltmail

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/alienscience/imapsrv"
	"time"
)

type basicMessage struct {
	flags        uint8
	internalDate time.Time
	size         uint32
	body         []byte
}

func (b *basicMessage) GobEncode() ([]byte, error) {
	w := new(bytes.Buffer)
	encoder := gob.NewEncoder(w)

	err := encoder.Encode(b.flags)
	if err != nil {
		return nil, err
	}
	err = encoder.Encode(b.internalDate)
	if err != nil {
		return nil, err
	}
	err = encoder.Encode(b.size)
	if err != nil {
		return nil, err
	}
	err = encoder.Encode(b.body)
	if err != nil {
		return nil, err
	}

	return w.Bytes(), err
}

func (b *basicMessage) GobDecode(byt []byte) error {
	r := bytes.NewBuffer(byt)
	dec := gob.NewDecoder(r)

	err := dec.Decode(b.flags)
	if err != nil {
		return err
	}
	err = dec.Decode(b.internalDate)
	if err != nil {
		return err
	}
	err = dec.Decode(b.size)
	if err != nil {
		return err
	}
	err = dec.Decode(b.body)
	if err != nil {
		return err
	}

	return nil
}

func (b *basicMessage) Flags() (uint8, error) {
	return b.flags, nil
}

func (b *basicMessage) InternalDate() (time.Time, error) {
	if b.internalDate.IsZero() {
		return b.internalDate, fmt.Errorf("timestamp unknown")
	} else {
		return b.internalDate, nil
	}
}

func (b *basicMessage) Size() (uint32, error) {
	if b.size == 0 {
		return 0, fmt.Errorf("size unknown")
	} else {
		return b.size, nil
	}
}

func (b *basicMessage) Reader() (imapsrv.MessageReader, error) {
	reader := newBinaryMessageReader(b.body)
	return reader, nil
}

type binaryMessageReader struct {
	data   []byte
	offset int64
}

func newBinaryMessageReader(data []byte) *binaryMessageReader {
	return &binaryMessageReader{data, 0}
}

func (b *binaryMessageReader) Close() error {
	return nil
}
func (b *binaryMessageReader) Seek(offset int64, whence int) (int64, error) {
	// 0 means relative to the origin of
	// the file, 1 means relative to the current offset, and 2 means
	// relative to the end.
	switch whence {
	case 0:
		b.offset = offset
	case 1:
		b.offset += offset
	case 2:
		b.offset = int64(len(b.data)) - offset
	default:
		return 0, fmt.Errorf("bad whence value: %d - expected 0, 1 or 2", whence)
	}
	return b.offset, nil
}
func (b *binaryMessageReader) Read(p []byte) (n int, err error) {
	p = b.data[b.offset:]
	return len(p), nil
}
