package gop

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Type of token
type Type int

const (
	// Nil type
	Nil Type = iota
	// Bool type
	Bool
	// Number type
	Number
	// Float type
	Float
	// Complex type
	Complex
	// String type
	String
	// Byte type
	Byte
	// RuneInt32 type
	RuneInt32
	// Chan type
	Chan
	// Func type
	Func
	// Error type
	Error

	// Comment type
	Comment

	// TypeName type
	TypeName

	// ParenOpen type
	ParenOpen
	// ParenClose type
	ParenClose

	// Dot type
	Dot
	// And type
	And

	// SliceOpen type
	SliceOpen
	// SliceItem type
	SliceItem
	// InlineComma type
	InlineComma
	// Comma type
	Comma
	// SliceClose type
	SliceClose

	// MapOpen type
	MapOpen
	// MapKey type
	MapKey
	// Colon type
	Colon
	// MapClose type
	MapClose

	// StructOpen type
	StructOpen
	// StructKey type
	StructKey
	// StructField type
	StructField
	// StructClose type
	StructClose
)

// Token represents a symbol in value layout. Build appends the
// token's rendered form to sb so the literal can be computed
// lazily and avoid allocating intermediate strings.
type Token interface {
	Type() Type
	Build(sb *strings.Builder)
}

// Lit is a token with a fixed literal string.
type Lit struct {
	T Type
	L string
}

// Type returns the token type.
func (l *Lit) Type() Type { return l.T }

// Build writes the literal to sb.
func (l *Lit) Build(sb *strings.Builder) { sb.WriteString(l.L) }

// IntTok lazily renders a signed integer as a Number token.
type IntTok int64

// Type returns Number.
func (IntTok) Type() Type { return Number }

// Build appends the decimal representation to sb.
func (n IntTok) Build(sb *strings.Builder) {
	var buf [20]byte
	sb.Write(strconv.AppendInt(buf[:0], int64(n), 10))
}

// UintTok lazily renders an unsigned integer as a Number token.
type UintTok uint64

// Type returns Number.
func (UintTok) Type() Type { return Number }

// Build appends the decimal representation to sb.
func (n UintTok) Build(sb *strings.Builder) {
	var buf [20]byte
	sb.Write(strconv.AppendUint(buf[:0], uint64(n), 10))
}

// Float64Tok lazily renders a float64 as a Number token, appending ".0" when the value is integral.
type Float64Tok float64

// Type returns Number.
func (Float64Tok) Type() Type { return Number }

// Build appends the formatted value to sb.
func (f Float64Tok) Build(sb *strings.Builder) {
	var buf [32]byte
	s := strconv.AppendFloat(buf[:0], float64(f), 'f', -1, 64)
	sb.Write(s)
	hasDot := false
	for _, b := range s {
		if b == '.' {
			hasDot = true
			break
		}
	}
	if !hasDot {
		sb.WriteString(".0")
	}
}

// FloatTok lazily renders a float at the given bit size as a Number token.
type FloatTok struct {
	V    float64
	Bits int
}

// Type returns Number.
func (FloatTok) Type() Type { return Number }

// Build appends the formatted value to sb.
func (f FloatTok) Build(sb *strings.Builder) {
	var buf [32]byte
	sb.Write(strconv.AppendFloat(buf[:0], f.V, 'f', -1, f.Bits))
}

// ComplexTok lazily renders a complex value at the given bit size as a Number token,
// stripping the parentheses that strconv.FormatComplex adds.
type ComplexTok struct {
	V    complex128
	Bits int
}

// Type returns Number.
func (ComplexTok) Type() Type { return Number }

// Build appends the formatted value to sb.
func (c ComplexTok) Build(sb *strings.Builder) {
	s := strconv.FormatComplex(c.V, 'f', -1, c.Bits)
	sb.WriteString(s[1 : len(s)-1])
}

// PtrTok lazily renders a uintptr as a Number token in 0xHEX form.
type PtrTok uintptr

// Type returns Number.
func (PtrTok) Type() Type { return Number }

// Build appends the hex representation to sb.
func (p PtrTok) Build(sb *strings.Builder) {
	sb.WriteString("0x")
	var buf [20]byte
	sb.Write(strconv.AppendUint(buf[:0], uint64(p), 16))
}

// CommentPtrTok lazily renders a uintptr as a Comment token in /* 0xHEX */ form.
type CommentPtrTok uintptr

// Type returns Comment.
func (CommentPtrTok) Type() Type { return Comment }

// Build appends the wrapped hex representation to sb.
func (p CommentPtrTok) Build(sb *strings.Builder) {
	sb.WriteString("/* 0x")
	var buf [20]byte
	sb.Write(strconv.AppendUint(buf[:0], uint64(p), 16))
	sb.WriteString(" */")
}

// RuneTok lazily renders a rune as a RuneInt32 token (quoted).
type RuneTok rune

// Type returns RuneInt32.
func (RuneTok) Type() Type { return RuneInt32 }

// Build appends the quoted rune to sb.
func (r RuneTok) Build(sb *strings.Builder) {
	sb.WriteString(strconv.QuoteRune(rune(r)))
}

// ByteTok lazily renders a byte as a Byte token, quoted when graphic else as 0xHEX.
type ByteTok byte

// Type returns Byte.
func (ByteTok) Type() Type { return Byte }

// Build appends the rendered byte to sb.
func (b ByteTok) Build(sb *strings.Builder) {
	r := rune(b)
	if unicode.IsGraphic(r) {
		sb.WriteString(strconv.QuoteRune(r))
		return
	}
	sb.WriteString("0x")
	var buf [4]byte
	sb.Write(strconv.AppendUint(buf[:0], uint64(b), 16))
}

// Pre-allocated singletons for common fixed-literal tokens. Reusing
// these avoids per-token heap allocations during tokenization.
var (
	tokParenOpen     Token = &Lit{ParenOpen, "("}
	tokParenClose    Token = &Lit{ParenClose, ")"}
	tokDot           Token = &Lit{Dot, "."}
	tokAnd           Token = &Lit{And, "&"}
	tokSliceOpen     Token = &Lit{SliceOpen, "{"}
	tokSliceClose    Token = &Lit{SliceClose, "}"}
	tokSliceItem     Token = &Lit{SliceItem, ""}
	tokInlineComma   Token = &Lit{InlineComma, ","}
	tokComma         Token = &Lit{Comma, ","}
	tokMapOpen       Token = &Lit{MapOpen, "{"}
	tokMapClose      Token = &Lit{MapClose, "}"}
	tokMapKey        Token = &Lit{MapKey, ""}
	tokColon         Token = &Lit{Colon, ":"}
	tokStructOpen    Token = &Lit{StructOpen, "{"}
	tokStructClose   Token = &Lit{StructClose, "}"}
	tokChan          Token = &Lit{Chan, "chan"}
	tokNil           Token = &Lit{Nil, "nil"}
	tokTrue          Token = &Lit{Bool, "true"}
	tokFalse         Token = &Lit{Bool, "false"}
	tokFuncMake      Token = &Lit{Func, "make"}
	tokFuncCircular  Token = &Lit{Func, SymbolCircular}
	tokFuncGopError  Token = &Lit{Func, SymbolGopError}
	tokFuncBase64    Token = &Lit{Func, SymbolBase64}
	tokFuncTime      Token = &Lit{Func, SymbolTime}
	tokFuncJSONStr   Token = &Lit{Func, SymbolJSONStr}
	tokFuncJSONBytes Token = &Lit{Func, SymbolJSONBytes}
	tokFuncPtr       Token = &Lit{Func, SymbolPtr}
	tokStrMaxDepth   Token = &Lit{String, "max depth exceeded"}
	tokTNGopRune     Token = &Lit{TypeName, "gop.Rune"}
	tokTNByte        Token = &Lit{TypeName, "byte"}
	tokTNBytes       Token = &Lit{TypeName, "[]byte"}
	tokTNUnsafePtr   Token = &Lit{TypeName, "unsafe.Pointer"}
	tokTNUintptr     Token = &Lit{TypeName, "uintptr"}
	tokTNDuration    Token = &Lit{TypeName, SymbolDuration}
)

// DefaultOptions for Tokenize.
var DefaultOptions = Options{
	MaxDepth: 15,
}

// Tokenize a random Go value with [DefaultOptions].
func Tokenize(v interface{}) []Token {
	return TokenizeWithOptions(v, DefaultOptions)
}

// Options controls tokenization.
type Options struct {
	// MaxDepth limits the depth of tokenization for nested structures.
	// If less than 1 there is no limit.
	MaxDepth int
}

// TokenizeWithOptions tokenizes v with the given Options.
func TokenizeWithOptions(v interface{}, opts Options) []Token {
	tz := tokenizer{Options: opts, global: map[uintptr]path{}, path: path{}}
	return tz.tokenize(reflect.ValueOf(v))
}

func tokenize(v reflect.Value) []Token {
	tz := tokenizer{global: map[uintptr]path{}, path: path{}}
	return tz.tokenize(v)
}

type path []interface{}

func (p path) tokens(opts Options) []Token {
	ts := []Token{}
	for i, seg := range p {
		ts = append(ts, TokenizeWithOptions(seg, opts)...)
		if i < len(p)-1 {
			ts = append(ts, tokInlineComma)
		}
	}
	return ts
}

type tokenizer struct {
	Options

	global map[uintptr]path
	path   path
}

func (tz *tokenizer) push(p interface{}) {
	if v := reflect.ValueOf(p); v.Kind() == reflect.Ptr {
		p = v.Pointer()
	}

	tz.path = append(tz.path, p)
}

func (tz *tokenizer) pop() {
	tz.path = tz.path[:len(tz.path)-1]
}

func (tz *tokenizer) circular(v reflect.Value) ([]Token, func()) {
	cleanup := func() {}

	switch v.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice:
		ptr := v.Pointer()
		if ptr == 0 {
			return nil, cleanup
		}

		if prev, has := tz.global[ptr]; has {
			ts := []Token{tokFuncCircular, tokParenOpen}
			ts = append(ts, prev.tokens(tz.Options)...)
			return append(ts, tokParenClose, tokDot,
				tokParenOpen, typeName(v.Type().String()), tokParenClose), cleanup
		}

		tz.global[ptr] = tz.path
		cleanup = func() {
			delete(tz.global, ptr)
		}
	}

	return nil, cleanup
}

func (tz *tokenizer) tokenize(v reflect.Value) []Token {
	if tz.MaxDepth > 0 && len(tz.path) >= tz.MaxDepth {
		return []Token{tokFuncGopError, tokParenOpen, tokStrMaxDepth, tokParenClose}
	}

	if ts, has := tz.tokenizeSpecial(v); has {
		return ts
	}

	{
		ts, cleanup := tz.circular(v)
		defer cleanup()

		if ts != nil {
			return ts
		}
	}

	switch v.Kind() {
	case reflect.Interface:
		return tz.tokenize(v.Elem())

	case reflect.Bool:
		if v.Bool() {
			return []Token{tokTrue}
		}
		return []Token{tokFalse}

	case reflect.String:
		return tokenizeString(v)

	case reflect.Chan:
		if v.IsNil() {
			return []Token{tokParenOpen,
				typeName(v.Type().String()), tokParenClose,
				tokParenOpen, tokNil, tokParenClose}
		}

		if v.Cap() == 0 {
			return []Token{tokFuncMake, tokParenOpen,
				tokChan, typeName(v.Type().Elem().String()), tokParenClose,
				CommentPtrTok(v.Pointer())}
		}
		return []Token{tokFuncMake, tokParenOpen, tokChan,
			typeName(v.Type().Elem().String()), tokInlineComma,
			IntTok(v.Cap()), tokParenClose,
			CommentPtrTok(v.Pointer())}

	case reflect.Func:
		if v.IsNil() {
			return []Token{tokParenOpen,
				typeName(v.Type().String()), tokParenClose,
				tokParenOpen, tokNil, tokParenClose}
		}

		return []Token{tokParenOpen, &Lit{TypeName, v.Type().String()},
			tokParenClose, tokParenOpen, tokNil, tokParenClose,
			CommentPtrTok(v.Pointer())}

	case reflect.Ptr:
		return tz.tokenizePtr(v)

	case reflect.UnsafePointer:
		return []Token{tokTNUnsafePtr, tokParenOpen, tokTNUintptr,
			tokParenOpen, PtrTok(v.Pointer()), tokParenClose, tokParenClose}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.Uintptr, reflect.Complex64, reflect.Complex128:
		return tokenizeNumber(v)

	default:
		// Slice, Array, Map, Struct — the remaining kinds reflect can surface here.
		return tz.tokenizeCollection(v)
	}
}

func (tz *tokenizer) tokenizeSpecial(v reflect.Value) ([]Token, bool) {
	if v.Kind() == reflect.Invalid {
		return []Token{tokNil}, true
	} else if r, ok := v.Interface().(rune); ok && unicode.IsGraphic(r) {
		return tokenizeRuneInt32(r), true
	} else if b, ok := v.Interface().(byte); ok {
		return tokenizeByte(b), true
	} else if t, ok := v.Interface().(time.Time); ok {
		return tokenizeTime(t), true
	} else if d, ok := v.Interface().(time.Duration); ok {
		return tokenizeDuration(d), true
	}

	return tz.tokenizeJSON(v)
}

func (tz *tokenizer) tokenizeCollection(v reflect.Value) []Token {
	var ts []Token

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.IsNil() {
			return []Token{typeName(v.Type().String()), tokParenOpen, tokNil, tokParenClose}
		}

		if data, ok := v.Interface().([]byte); ok {
			ts = tokenizeBytes(data)
			break
		}
		ts = make([]Token, 0, v.Len()*4+3)
		ts = append(ts, typeName(v.Type().String()))
		ts = append(ts, tokSliceOpen)
		for i := 0; i < v.Len(); i++ {
			el := v.Index(i)
			ts = append(ts, tokSliceItem)
			tz.push(i)
			ts = append(ts, tz.tokenize(el)...)
			tz.pop()
			ts = append(ts, tokComma)
		}
		ts = append(ts, tokSliceClose)

	case reflect.Map:
		if v.IsNil() {
			return []Token{typeName(v.Type().String()), tokParenOpen, tokNil, tokParenClose}
		}

		ts = make([]Token, 0, v.Len()*6+3)
		ts = append(ts, typeName(v.Type().String()))
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool {
			return compare(keys[i].Interface(), keys[j].Interface()) < 0
		})
		ts = append(ts, tokMapOpen)
		for _, k := range keys {
			ts = append(ts, tokMapKey)

			if k.Kind() == reflect.Interface && k.Elem().Kind() == reflect.Ptr {
				ts = append(ts, tokenizeMapKey(k)...)
			} else {
				ts = append(ts, tokenize(k)...)
			}

			ts = append(ts, tokColon)
			tz.push(k.Interface())
			ts = append(ts, tz.tokenize(v.MapIndex(k))...)
			tz.pop()
			ts = append(ts, tokComma)
		}
		ts = append(ts, tokMapClose)

	case reflect.Struct:
		t := v.Type()

		ts = make([]Token, 0, v.NumField()*6+3)
		ts = append(ts, typeName(t.String()))
		ts = append(ts, tokStructOpen)
		for i := 0; i < v.NumField(); i++ {
			name := t.Field(i).Name
			ts = append(ts, tokStructKey)
			ts = append(ts, &Lit{StructField, name})

			f := v.Field(i)
			if !f.CanInterface() {
				f = GetPrivateField(v, i)
			}
			ts = append(ts, tokColon)
			tz.push(name)
			ts = append(ts, tz.tokenize(f)...)
			tz.pop()
			ts = append(ts, tokComma)
		}
		ts = append(ts, tokStructClose)
	}

	return ts
}

var tokStructKey Token = &Lit{StructKey, ""}

func tokenizeNumber(v reflect.Value) []Token {
	tName := v.Type().String()

	switch v.Kind() {
	case reflect.Int:
		if tName != "int" {
			return []Token{typeName(tName), tokParenOpen, IntTok(v.Int()), tokParenClose}
		}
		return []Token{IntTok(v.Int())}

	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return []Token{typeName(tName), tokParenOpen, IntTok(v.Int()), tokParenClose}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return []Token{typeName(tName), tokParenOpen, UintTok(v.Uint()), tokParenClose}

	case reflect.Float32:
		return []Token{typeName(tName), tokParenOpen, FloatTok{V: v.Float(), Bits: 32}, tokParenClose}

	case reflect.Float64:
		if tName != "float64" {
			return []Token{typeName(tName), tokParenOpen, Float64Tok(v.Float()), tokParenClose}
		}
		return []Token{Float64Tok(v.Float())}

	case reflect.Complex64:
		return []Token{typeName(tName), tokParenOpen, ComplexTok{V: v.Complex(), Bits: 64}, tokParenClose}

	default:
		// reflect.Complex128 — callers dispatch only numeric kinds here.
		if tName != "complex128" {
			return []Token{typeName(tName), tokParenOpen, ComplexTok{V: v.Complex(), Bits: 128}, tokParenClose}
		}
		return []Token{ComplexTok{V: v.Complex(), Bits: 128}}
	}
}

func tokenizeRuneInt32(r rune) []Token {
	return []Token{
		tokTNGopRune,
		tokParenOpen,
		IntTok(int64(r)),
		tokInlineComma,
		RuneTok(r),
		tokParenClose,
	}
}

func tokenizeByte(b byte) []Token {
	return []Token{tokTNByte, tokParenOpen, ByteTok(b), tokParenClose}
}

func tokenizeTime(t time.Time) []Token {
	ext := GetPrivateFieldByName(reflect.ValueOf(t), "ext").Int()
	return []Token{tokFuncTime, tokParenOpen,
		&Lit{String, t.Format(time.RFC3339Nano)},
		tokInlineComma, IntTok(ext), tokParenClose}
}

func tokenizeDuration(d time.Duration) []Token {
	return []Token{tokTNDuration, tokParenOpen,
		&Lit{String, d.String()}, tokParenClose}
}

func tokenizeString(v reflect.Value) []Token {
	return []Token{&Lit{String, v.String()}}
}

func tokenizeBytes(data []byte) []Token {
	if utf8.Valid(data) {
		return []Token{tokTNBytes, tokParenOpen,
			&Lit{String, string(data)}, tokParenClose}
	}
	return []Token{tokFuncBase64, tokParenOpen,
		&Lit{String, base64.StdEncoding.EncodeToString(data)}, tokParenClose}
}

func tokenizeMapKey(v reflect.Value) []Token {
	return []Token{
		tokParenOpen, typeName(v.Type().String()), tokParenClose,
		tokParenOpen, tokNil, tokParenClose,
		CommentPtrTok(v.Elem().Pointer()),
	}
}

func (tz *tokenizer) tokenizePtr(v reflect.Value) []Token {
	if v.Elem().Kind() == reflect.Invalid {
		return []Token{
			tokParenOpen, typeName(v.Type().String()), tokParenClose,
			tokParenOpen, tokNil, tokParenClose}
	}

	needFn := false

	switch v.Elem().Kind() {
	case reflect.Struct, reflect.Map, reflect.Slice, reflect.Array:
		if _, ok := v.Elem().Interface().([]byte); ok {
			needFn = true
		}
	default:
		needFn = true
	}

	if needFn {
		ts := []Token{tokFuncPtr, tokParenOpen}
		ts = append(ts, tz.tokenize(v.Elem())...)
		ts = append(ts, tokParenClose, tokDot, tokParenOpen,
			typeName(v.Type().String()), tokParenClose)
		return ts
	}
	ts := []Token{tokAnd}
	ts = append(ts, tz.tokenize(v.Elem())...)
	return ts
}

func (tz *tokenizer) tokenizeJSON(v reflect.Value) ([]Token, bool) {
	var jv interface{}
	ts := []Token{}
	s := ""
	if v.Kind() == reflect.String {
		s = v.String()
		err := json.Unmarshal([]byte(s), &jv)
		if err != nil {
			return nil, false
		}
		ts = append(ts, tokFuncJSONStr)
	} else if b, ok := v.Interface().([]byte); ok {
		err := json.Unmarshal(b, &jv)
		if err != nil {
			return nil, false
		}
		s = string(b)
		ts = append(ts, tokFuncJSONBytes)
	}

	_, isObj := jv.(map[string]interface{})
	_, isArr := jv.([]interface{})

	if isObj || isArr {
		ts = append(ts, tokParenOpen)
		ts = append(ts, TokenizeWithOptions(jv, tz.Options)...)
		ts = append(ts, tokInlineComma,
			&Lit{String, s}, tokParenClose)
		return ts, true
	}

	return nil, false
}

func typeName(t string) Token {
	return &Lit{TypeName, t}
}
