package torrent

import (
	"fmt"
	"io"
	"strings"
)

type BencodeType int

const (
	DICT BencodeType = iota
	LIST
	INT
	STRING
	END
)

type pair struct {
	key   string
	value bencodeObject
}
type bencodeObject struct {
	objType BencodeType
	dict    []pair
	list    []bencodeObject
	val     int64
	str     string
}

func (b *bencodeObject) Marshal() (string, error) {
	var builder strings.Builder
	err := b.marshalRecursive(&builder)
	if err != nil {
		return "", err
	}
	return builder.String(), nil
}
func (b *bencodeObject) marshalRecursive(builder *strings.Builder) error {
	switch b.objType {
	case STRING:
		fmt.Fprintf(builder, "%d:%s", len(b.str), b.str)
	case INT:
		fmt.Fprintf(builder, "i%de", b.val)
	case LIST:
		builder.WriteByte('l')
		for _, item := range b.list {
			if err := item.marshalRecursive(builder); err != nil {
				return err
			}
		}
		builder.WriteByte('e')
	case DICT:
		builder.WriteByte('d')
		for _, p := range b.dict {
			fmt.Fprintf(builder, "%d:%s", len(p.key), p.key)
			if err := p.value.marshalRecursive(builder); err != nil {
				return err
			}
		}
		builder.WriteByte('e')
	default:
		return fmt.Errorf("invalid type")
	}
	return nil
}
func (b *bencodeObject) valAt(key string) (bencodeObject, error) {
	if b.objType != DICT {
		return bencodeObject{}, fmt.Errorf("not a dictionary")
	}
	for _, p := range b.dict {
		if p.key == key {
			return p.value, nil
		}
	}
	return bencodeObject{}, fmt.Errorf("key not found")
}
func (b *bencodeObject) valAtIndex(index int) (bencodeObject, error) {
	if b.objType != LIST {
		return bencodeObject{}, fmt.Errorf("not a list")
	}
	if index < 0 || index >= len(b.list) {
		return bencodeObject{}, fmt.Errorf("index out of bounds")
	}
	return b.list[index], nil
}
func readOneByte(r io.Reader) int {
	buf := make([]byte, 1)
	n, err := r.Read(buf)
	if n == 0 || err != nil {
		return -1
	}
	return int(buf[0])
}
func Unmarshal(r io.Reader, ben *bencodeObject) error {
	if ben == nil {
		return fmt.Errorf("cant unmarshal objects into nil object")
	}
	char := readOneByte(r)
	if char == -1 {
		return fmt.Errorf("couldnt unmarshal object")
	}
	if char >= int('0') && char <= int('9') {
		ben.objType = STRING
		len := int(char) - int('0')
		for {
			char = readOneByte(r)
			if char == int(':') {
				break
			} else if char >= int('0') && char <= int('9') {
				len = len*10 + (char - int('0'))
			} else {
				return fmt.Errorf("couldnt unmarshal object")
			}
		}
		buf := make([]byte, len)
		n, err := io.ReadFull(r, buf)
		if err != nil || n != len {
			return fmt.Errorf("couldnt unmarshal object")
		}
		ben.str = string(buf)
	} else if char == 'd' {
		ben.objType = DICT
		for {
			key := bencodeObject{}
			err := Unmarshal(r, &key)
			if err != nil {
				return err
			}
			if key.objType == END {
				break
			}
			value := bencodeObject{}
			err = Unmarshal(r, &value)
			if err != nil {
				return err
			}
			ben.dict = append(ben.dict, pair{
				key:   key.str,
				value: value,
			})
		}
	} else if char == 'l' {
		ben.objType = LIST
		for {
			key := bencodeObject{}
			err := Unmarshal(r, &key)
			if err != nil {
				return err
			}
			if key.objType == END {
				break
			}
			ben.list = append(ben.list, key)
		}
	} else if char == 'i' {
		ben.objType = INT
		char := readOneByte(r)
		sign := int64(1)
		num := int64(0)
		if char == '-' {
			sign = int64(-1)
			char = readOneByte(r)
		}
		for char != 'e' {
			if char < '0' || char > '9' {
				return fmt.Errorf("couldnt unmarshal object")
			}
			num = num*10 + int64(char-'0')
			char = readOneByte(r)
		}
		ben.val = num * sign
	} else if char == 'e' {
		ben.objType = END
	}
	return nil
}
