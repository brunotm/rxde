package rxde

import (
	"bytes"
)

// naive and fast json value appender for flat json documents
// assumes well formated values and it doesn't handle espcaping for keys
// o no ',",\ or control characters
func appendJSON(data []byte, key string, value []byte) (newData []byte) {
	dsz := len(data)
	buf := new(bytes.Buffer)

	if dsz == 0 {
		data = make([]byte, 0, 64)
		data = append(data, '{', '}')
		dsz = 2
	}

	if dsz > 2 {
		buf.WriteByte(',')
	}

	buf.WriteByte('"')
	buf.WriteString(key)
	buf.WriteByte('"')
	buf.WriteByte(':')
	buf.Write(value)

	return append(data[:dsz-1], append(buf.Bytes(), data[dsz-1:]...)...)
}
