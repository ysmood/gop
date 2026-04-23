package gop

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Style type
type Style struct {
	Set   string
	Unset string
}

var (
	// Bold style
	Bold = addStyle(1, 22)
	// Faint style
	Faint = addStyle(2, 22)
	// Italic style
	Italic = addStyle(3, 23)
	// Underline style
	Underline = addStyle(4, 24)
	// Blink style
	Blink = addStyle(5, 25)
	// RapidBlink style
	RapidBlink = addStyle(6, 26)
	// Invert style
	Invert = addStyle(7, 27)
	// Hide style
	Hide = addStyle(8, 28)
	// Strike style
	Strike = addStyle(9, 29)

	// Black color
	Black = addStyle(30, 39)
	// Red color
	Red = addStyle(31, 39)
	// Green color
	Green = addStyle(32, 39)
	// Yellow color
	Yellow = addStyle(33, 39)
	// Blue color
	Blue = addStyle(34, 39)
	// Magenta color
	Magenta = addStyle(35, 39)
	// Cyan color
	Cyan = addStyle(36, 39)
	// White color
	White = addStyle(37, 39)

	// BgBlack color
	BgBlack = addStyle(40, 49)
	// BgRed color
	BgRed = addStyle(41, 49)
	// BgGreen color
	BgGreen = addStyle(42, 49)
	// BgYellow color
	BgYellow = addStyle(43, 49)
	// BgBlue color
	BgBlue = addStyle(44, 49)
	// BgMagenta color
	BgMagenta = addStyle(45, 49)
	// BgCyan color
	BgCyan = addStyle(46, 49)
	// BgWhite color
	BgWhite = addStyle(47, 49)

	// None type
	None = Style{}
)

// Styled wraps an inner Token and applies the given Styles when built.
// It is the token form of the legacy Stylize helper.
type Styled struct {
	Inner  Token
	Styles []Style
}

// Type returns the inner token type.
func (s Styled) Type() Type {
	if s.Inner == nil {
		return Nil
	}
	return s.Inner.Type()
}

// Build writes the styled rendering of the inner token to sb.
func (s Styled) Build(sb *strings.Builder) {
	if s.Inner == nil {
		return
	}
	if NoStyle || !hasActiveStyle(s.Styles) {
		s.Inner.Build(sb)
		return
	}

	// Fast path: a Lit already holds its string, skip the temp builder.
	if l, ok := s.Inner.(*Lit); ok {
		Render(sb, l.L, s.Styles)
		return
	}

	var inner strings.Builder
	s.Inner.Build(&inner)
	Render(sb, inner.String(), s.Styles)
}

func hasActiveStyle(styles []Style) bool {
	for _, s := range styles {
		if s != None {
			return true
		}
	}
	return false
}

// Render writes the stylized form of str to sb without allocating
// intermediate strings: both the single-line and multi-line paths stream
// directly into sb.
func Render(sb *strings.Builder, str string, styles []Style) {
	if NoStyle || !hasActiveStyle(styles) {
		sb.WriteString(str)
		return
	}
	if !strings.ContainsAny(str, "\r\n") {
		writeStyleSets(sb, styles)
		sb.WriteString(str)
		writeStyleUnsets(sb, styles)
		return
	}

	newline := firstNewline(str)
	remaining := str
	first := true
	for {
		idx := strings.IndexByte(remaining, '\n')
		if idx < 0 {
			if !first {
				sb.WriteString(newline)
			}
			writeStyleSets(sb, styles)
			sb.WriteString(remaining)
			writeStyleUnsets(sb, styles)
			return
		}
		end := idx
		if idx > 0 && remaining[idx-1] == '\r' {
			end = idx - 1
		}
		if !first {
			sb.WriteString(newline)
		}
		first = false
		writeStyleSets(sb, styles)
		sb.WriteString(remaining[:end])
		writeStyleUnsets(sb, styles)
		remaining = remaining[idx+1:]
	}
}

func writeStyleSets(sb *strings.Builder, styles []Style) {
	for i := len(styles) - 1; i >= 0; i-- {
		if styles[i] != None {
			sb.WriteString(styles[i].Set)
		}
	}
}

func writeStyleUnsets(sb *strings.Builder, styles []Style) {
	for _, s := range styles {
		if s != None {
			sb.WriteString(s.Unset)
		}
	}
}

func firstNewline(s string) string {
	idx := strings.IndexByte(s, '\n')
	if idx > 0 && s[idx-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}

// S is the shortcut for Stylize.
func S(str string, styles ...Style) string {
	return Stylize(str, styles)
}

// Stylize wraps str with the given styles.
func Stylize(str string, styles []Style) string {
	if NoStyle || !hasActiveStyle(styles) {
		return str
	}
	var sb strings.Builder
	Render(&sb, str, styles)
	return sb.String()
}

// NoStyle respects https://no-color.org/ and "tput colors"
var NoStyle = func() bool {
	_, noColor := os.LookupEnv("NO_COLOR")

	b, _ := exec.Command("tput", "colors").CombinedOutput()
	n, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 32)
	return noColor || n == 0
}()

// RegANSI token
var RegANSI = regexp.MustCompile(`\x1b\[\d+m`)

// StripANSI tokens
func StripANSI(str string) string {
	return RegANSI.ReplaceAllString(str, "")
}

var regNum = regexp.MustCompile(`\d+`)

// VisualizeANSI tokens
func VisualizeANSI(str string) string {
	return RegANSI.ReplaceAllStringFunc(str, func(s string) string {
		return "<" + regNum.FindString(s) + ">"
	})
}

// FixNestedStyle like
//
//	<d><a>1<b>2<c>3</d></>4</>5</>
//
// into
//
//	<d><a>1</><b>2</><c>3</d></><b>4</><a>5</>
func FixNestedStyle(s string) string {
	out := ""
	stacks := map[string][]string{}
	i := 0
	l := 0
	r := 0

	for i < len(s) {
		loc := RegANSI.FindStringIndex(s[i:])
		if loc == nil {
			break
		}

		l, r = i+loc[0], i+loc[1]
		token := s[l:r]

		out += s[i:l]

		unset := GetStyle(token).Unset

		if unset == "" {
			unset = token
		}

		if _, has := stacks[unset]; !has {
			stacks[unset] = []string{}
		}

		stack := stacks[unset]
		if len(stack) == 0 {
			stack = append(stack, token)
			out += token
		} else {
			if token == GetStyle(last(stack)).Unset {
				out += token
				stack = stack[:len(stack)-1]
				if len(stack) > 0 {
					out += last(stack)
				}
			} else {
				out += GetStyle(last(stack)).Unset
				stack = append(stack, token)
				out += token
			}
		}
		stacks[unset] = stack

		i = r
	}

	return out + s[i:]
}

// GetStyle from available styles
func GetStyle(s string) Style {
	return styleSetMap[s]
}

func last(list []string) string {
	return list[len(list)-1]
}

var styleSetMap = map[string]Style{}

func addStyle(set, unset int) Style {
	s := Style{
		fmt.Sprintf("\x1b[%dm", set),
		fmt.Sprintf("\x1b[%dm", unset),
	}
	styleSetMap[s.Set] = s
	return s
}
