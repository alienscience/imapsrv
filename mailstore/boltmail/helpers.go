package boltmail

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"strconv"

	"github.com/boltdb/bolt"
)

func toBytes(i interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(i)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func fromBytes(b []byte, i interface{}) error {
	buf := bytes.NewBuffer(b)
	dec := gob.NewDecoder(buf)
	return dec.Decode(i)
}

func getInt(key []byte, buck *bolt.Bucket) (int, error) {
	b := buck.Get(key)
	if len(b) == 0 {
		return 0, fmt.Errorf("key did not exist")
	}
	return strconv.Atoi(string(b))
}
