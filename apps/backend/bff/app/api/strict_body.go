package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf16"
)

var errShape = errors.New("api: the body does not hold exactly the members of the contract")

type shapeKind int

const (
	scalarShape shapeKind = iota
	objectShape
	listShape
)

type shape struct {
	kind     shapeKind
	members  map[string]member
	elem     *shape
	maxItems int
}

type member struct {
	bit   uint64
	shape *shape
}

var shapes sync.Map

func mustShapeOf(typ reflect.Type) *shape {
	if known, ok := shapes.Load(typ); ok {
		return known.(*shape)
	}
	compiled, err := compileShape(typ)
	if err != nil {
		panic(err)
	}
	stored, _ := shapes.LoadOrStore(typ, compiled)
	return stored.(*shape)
}

func compileShape(typ reflect.Type) (*shape, error) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return &shape{kind: scalarShape}, nil
	}
	if typ.NumField() > 64 {
		return nil, fmt.Errorf("api: %s has %d fields, more than the 64 bits that track the members seen", typ, typ.NumField())
	}
	compiled := &shape{kind: objectShape, members: make(map[string]member, typ.NumField())}
	for i := range typ.NumField() {
		field := typ.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if !field.IsExported() || name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		value, err := compileField(typ, field)
		if err != nil {
			return nil, err
		}
		compiled.members[name] = member{bit: 1 << uint(i), shape: value}
	}
	return compiled, nil
}

func compileField(owner reflect.Type, field reflect.StructField) (*shape, error) {
	if field.Type.Kind() != reflect.Slice && field.Type.Kind() != reflect.Array {
		return compileShape(field.Type)
	}
	maxItems, err := strconv.Atoi(field.Tag.Get("maxItems"))
	if err != nil || maxItems < 1 {
		return nil, fmt.Errorf("api: %s.%s is a list without the maxItems of its schema", owner, field.Name)
	}
	elem, err := compileShape(field.Type.Elem())
	if err != nil {
		return nil, err
	}
	return &shape{kind: listShape, elem: elem, maxItems: maxItems}, nil
}

// WHY: encoding/json matches member names ignoring case, keeps the last of a
// repeated member and binds null as absence; the contract admits none of them.
func exactShape(raw []byte, of *shape) error {
	s := scanner{raw: raw}
	return s.value(of)
}

type scanner struct {
	raw []byte
	at  int
}

func (s *scanner) space() {
	for s.at < len(s.raw) {
		switch s.raw[s.at] {
		case ' ', '\t', '\n', '\r':
			s.at++
		default:
			return
		}
	}
}

func (s *scanner) next() byte {
	s.space()
	if s.at >= len(s.raw) {
		return 0
	}
	return s.raw[s.at]
}

func (s *scanner) value(of *shape) error {
	first := s.next()
	switch of.kind {
	case objectShape:
		if first != '{' {
			return errShape
		}
		return s.object(of)
	case listShape:
		if first != '[' {
			return errShape
		}
		return s.list(of)
	default:
		return s.scalar(first)
	}
}

func (s *scanner) object(of *shape) error {
	s.at++
	if s.next() == '}' {
		s.at++
		return nil
	}
	var seen uint64
	for {
		if s.next() != '"' {
			return errShape
		}
		name, err := s.key()
		if err != nil {
			return err
		}
		declared, ok := of.members[string(name)]
		if !ok || seen&declared.bit != 0 {
			return errShape
		}
		seen |= declared.bit
		if s.next() != ':' {
			return errShape
		}
		s.at++
		if err := s.value(declared.shape); err != nil {
			return err
		}
		switch s.next() {
		case ',':
			s.at++
		case '}':
			s.at++
			return nil
		default:
			return errShape
		}
	}
}

func (s *scanner) list(of *shape) error {
	s.at++
	if s.next() == ']' {
		s.at++
		return nil
	}
	for items := 1; ; items++ {
		if items > of.maxItems {
			return errShape
		}
		if err := s.value(of.elem); err != nil {
			return err
		}
		switch s.next() {
		case ',':
			s.at++
		case ']':
			s.at++
			return nil
		default:
			return errShape
		}
	}
}

func (s *scanner) scalar(first byte) error {
	switch first {
	case 0, '{', '[', 'n', ',', ':', '}', ']':
		return errShape
	case '"':
		_, _, err := s.text()
		return err
	}
	for s.at < len(s.raw) {
		switch s.raw[s.at] {
		case ',', '}', ']', ' ', '\t', '\n', '\r':
			return nil
		}
		s.at++
	}
	return nil
}

func (s *scanner) key() ([]byte, error) {
	start := s.at
	content, escaped, err := s.text()
	if err != nil || !escaped {
		return content, err
	}
	var name string
	if err := json.Unmarshal(s.raw[start:s.at], &name); err != nil {
		return nil, errShape
	}
	return []byte(name), nil
}

func (s *scanner) text() (content []byte, escaped bool, err error) {
	start := s.at + 1
	for from := start; ; {
		end := bytes.IndexByte(s.raw[from:], '"')
		if end < 0 {
			return nil, false, errShape
		}
		end += from
		backslashes := 0
		for i := end - 1; i >= start && s.raw[i] == '\\'; i-- {
			backslashes++
		}
		if backslashes%2 == 0 {
			s.at = end + 1
			content = s.raw[start:end]
			return content, bytes.IndexByte(content, '\\') >= 0, nil
		}
		from = end + 1
	}
}

// WHY: encoding/json decodes an unpaired \uD800-\uDFFF escape as U+FFFD, so a
// body that is not Unicode would reach the context as another text.
func unpairedSurrogate(raw []byte) bool {
	for i := 0; i < len(raw); {
		found := bytes.IndexByte(raw[i:], '\\')
		if found < 0 {
			return false
		}
		i += found
		r, escaped := escapeAt(raw, i)
		if !escaped {
			i += 2
			continue
		}
		i += 6
		if !utf16.IsSurrogate(r) {
			continue
		}
		low, escaped := escapeAt(raw, i)
		if !escaped || utf16.DecodeRune(r, low) == unicode.ReplacementChar {
			return true
		}
		i += 6
	}
	return false
}

func escapeAt(raw []byte, at int) (rune, bool) {
	if at+6 > len(raw) || raw[at] != '\\' || raw[at+1] != 'u' {
		return 0, false
	}
	var code rune
	for _, digit := range raw[at+2 : at+6] {
		value, ok := hexValue(digit)
		if !ok {
			return 0, false
		}
		code = code<<4 | value
	}
	return code, true
}

func hexValue(digit byte) (rune, bool) {
	switch {
	case '0' <= digit && digit <= '9':
		return rune(digit - '0'), true
	case 'a' <= digit && digit <= 'f':
		return rune(digit-'a') + 10, true
	case 'A' <= digit && digit <= 'F':
		return rune(digit-'A') + 10, true
	}
	return 0, false
}
