package boltmail

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"

	"github.com/alienscience/imapsrv"
	"github.com/tdewolff/buffer"
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

	err := dec.Decode(&b.flags)
	if err != nil {
		return err
	}
	err = dec.Decode(&b.internalDate)
	if err != nil {
		return err
	}
	err = dec.Decode(&b.size)
	if err != nil {
		return err
	}
	err = dec.Decode(&b.body)
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
	reader := buffer.NewReader(b.body)
	return reader, nil
}
