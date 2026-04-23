// Package gop ...
package gop

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Stdout is the default stdout for gop.P .
var Stdout io.Writer = os.Stdout

const indentUnit = "    "

// Theme to color values
type Theme func(t Type) []Style

// ThemeDefault colors for Sprint
var ThemeDefault = func(t Type) []Style {
	switch t {
	case TypeName:
		return []Style{Cyan}
	case Bool, Chan:
		return []Style{Blue}
	case RuneInt32, Byte, String:
		return []Style{Yellow}
	case Number:
		return []Style{Green}
	case Func:
		return []Style{Magenta}
	case Comment:
		return []Style{White}
	case Nil:
		return []Style{Red}
	case Error:
		return []Style{Underline, Red}
	default:
		return []Style{None}
	}
}

// ThemeNone colors for Sprint
var ThemeNone = func(t Type) []Style {
	return []Style{None}
}

// F is a shortcut for Format with color
func F(v interface{}) string {
	return Format(Tokenize(v), ThemeDefault)
}

// P pretty print the values
func P(values ...interface{}) error {
	list := []interface{}{}
	for _, v := range values {
		list = append(list, F(v))
	}

	pc, file, line, _ := runtime.Caller(1)
	fn := runtime.FuncForPC(pc).Name()
	cwd, _ := os.Getwd()
	file, _ = filepath.Rel(cwd, file)
	tpl := Stylize("// %s %s:%d (%s)\n", ThemeDefault(Comment))
	_, _ = fmt.Fprintf(Stdout, tpl, time.Now().Format(time.RFC3339Nano), file, line, fn)

	_, err := fmt.Fprintln(Stdout, list...)
	return err
}

// Plain is a shortcut for Format with plain color
func Plain(v interface{}) string {
	return Format(Tokenize(v), ThemeNone)
}

// indentCache holds pre-repeated indent strings to avoid calling
// strings.Repeat for every indented line.
var indentCache = func() []string {
	out := make([]string, 33)
	for i := range out {
		out[i] = strings.Repeat(indentUnit, i)
	}
	return out
}()

func writeIndent(sb *strings.Builder, depth int) {
	if depth < len(indentCache) {
		sb.WriteString(indentCache[depth])
		return
	}
	for i := 0; i < depth; i++ {
		sb.WriteString(indentUnit)
	}
}

// Format a list of tokens.
func Format(ts []Token, theme Theme) string {
	var out strings.Builder
	depth := 0
	for i, t := range ts {
		tt := t.Type()
		if oneOf(tt, SliceOpen, MapOpen, StructOpen) {
			depth++
		}
		if i < len(ts)-1 && oneOf(ts[i+1].Type(), SliceClose, MapClose, StructClose) {
			depth--
		}

		styles := theme(tt)

		switch tt {
		case SliceOpen, MapOpen, StructOpen:
			buildStyled(&out, t, styles)
			out.WriteByte('\n')
		case SliceItem, MapKey, StructKey:
			writeIndent(&out, depth)
		case Colon, InlineComma, Chan:
			buildStyled(&out, t, styles)
			out.WriteByte(' ')
		case Comma:
			buildStyled(&out, t, styles)
			out.WriteByte('\n')
		case SliceClose, MapClose, StructClose:
			s := out.String()
			if strings.HasSuffix(s, "{\n") {
				out.Reset()
				out.WriteString(s[:len(s)-1])
				buildStyled(&out, t, styles)
			} else {
				writeIndent(&out, depth)
				buildStyled(&out, t, styles)
			}
		case String:
			writeReadableString(&out, t, depth, styles)
		default:
			buildStyled(&out, t, styles)
		}
	}

	return out.String()
}

// buildStyled renders t into sb, applying styles when any are active.
// Non-Lit tokens always produce single-line output, so we can emit the
// escape sequences around t.Build directly and skip the temp builder.
func buildStyled(sb *strings.Builder, t Token, styles []Style) {
	if NoStyle || !hasActiveStyle(styles) {
		t.Build(sb)
		return
	}
	if l, ok := t.(*Lit); ok {
		Render(sb, l.L, styles)
		return
	}
	for i := len(styles) - 1; i >= 0; i-- {
		if styles[i] != None {
			sb.WriteString(styles[i].Set)
		}
	}
	t.Build(sb)
	for _, s := range styles {
		if s != None {
			sb.WriteString(s.Unset)
		}
	}
}

// writeReadableString handles the String-type token path: it materializes
// the raw literal, reshapes it via readableStr, then stylizes the result.
func writeReadableString(sb *strings.Builder, t Token, depth int, styles []Style) {
	var raw string
	if l, ok := t.(*Lit); ok {
		raw = l.L
	} else {
		var inner strings.Builder
		t.Build(&inner)
		raw = inner.String()
	}
	s := readableStr(depth, raw)
	if NoStyle || !hasActiveStyle(styles) {
		sb.WriteString(s)
		return
	}
	Render(sb, s, styles)
}

func oneOf(t Type, list ...Type) bool {
	for _, el := range list {
		if t == el {
			return true
		}
	}
	return false
}

// To make multi-line string block more human readable.
// Split newline into two strings, convert "\t" into tab.
// Such as format string: "line one \n\t line two" into:
//
//	"line one \n" +
//	"	 line two"
func readableStr(depth int, s string) string {
	if (strings.Contains(s, "\n") || strings.Contains(s, `"`)) && !strings.Contains(s, "`") {
		return "`" + s + "`"
	}

	s = fmt.Sprintf("%#v", s)
	s, _ = replaceEscaped(s, 't', "	")

	indent := strings.Repeat(indentUnit, depth+1)
	if n, has := replaceEscaped(s, 'n', "\\n\" +\n"+indent+"\""); has {
		return "\"\" +\n" + indent + n
	}

	return s
}

// We use a simple state machine to replace escaped char like "\n"
func replaceEscaped(s string, escaped rune, new string) (string, bool) {
	type State int
	const (
		init State = iota
		preMatch
		match
	)

	state := init
	var out strings.Builder
	var buf strings.Builder
	has := false

	onInit := func(r rune) {
		state = init
		out.WriteString(buf.String())
		out.WriteRune(r)
		buf.Reset()
	}

	onPreMatch := func() {
		state = preMatch
		buf.Reset()
		buf.WriteString("\\")
	}

	onEscape := func() {
		state = match
		out.WriteString(new)
		buf.Reset()
		has = true
	}

	for _, r := range s {
		switch state {
		case preMatch:
			switch r {
			case escaped:
				onEscape()
			default:
				onInit(r)
			}

		case match:
			switch r {
			case '\\':
				onPreMatch()
			default:
				onInit(r)
			}

		default:
			switch r {
			case '\\':
				onPreMatch()
			default:
				onInit(r)
			}
		}
	}

	return out.String(), has
}
