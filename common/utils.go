package common

import (
    "encoding/binary"
    "fmt"
)

// Write a 2-byte length + string data.
func writeString(buf []byte, s string) []byte {
    strBytes := []byte(s)
    length := uint16(len(strBytes))
    lenBuf := make([]byte, 2)
    binary.BigEndian.PutUint16(lenBuf, length)

    buf = append(buf, lenBuf...)
    buf = append(buf, strBytes...)
    return buf
}

// Read a 2-byte length + string data.
func readString(data []byte, offset int) (string, int, error) {
    if offset+2 > len(data) {
        return "", offset, fmt.Errorf("not enough bytes to read string length")
    }
    length := binary.BigEndian.Uint16(data[offset : offset+2])
    offset += 2

    if offset+int(length) > len(data) {
        return "", offset, fmt.Errorf("not enough bytes for string content")
    }

    strBytes := data[offset : offset+int(length)]
    offset += int(length)
    return string(strBytes), offset, nil
}
