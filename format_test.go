package gop_test

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"text/template"
	"time"
	"unsafe"

	"github.com/ysmood/gop"
)

func eq(t *testing.T, a, b interface{}) {
	t.Helper()
	if a == b {
		return
	}
	t.Log(a, "[should equal]", b)
	t.Fail()
}

func neq(t *testing.T, a, b interface{}) {
	if a != b {
		return
	}
	t.Log("should not equal")
	t.Fail()
}

// RandStr generates a random string with the specified length
func randStr(l int) string {
	b := make([]byte, (l+1)/2)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:l]
}

func render(value string, data interface{}) string {
	out := bytes.NewBuffer(nil)
	t := template.New("")
	t, err := t.Parse(value)
	if err != nil {
		panic(err)
	}
	err = t.Execute(out, data)
	if err != nil {
		panic(err)
	}
	return out.String()
}

func assertPanic(t *testing.T, fn func()) (val interface{}) {
	t.Helper()

	defer func() {
		t.Helper()

		val = recover()
		if val == nil {
			t.Error("should panic")
		}
	}()

	fn()

	return
}

func TestStyle(t *testing.T) {
	s := gop.Style{Set: "<s>", Unset: "</s>"}

	eq(t, gop.S("test", s), "<s>test</s>")
	eq(t, gop.S("", s), "<s></s>")
	eq(t, gop.S("", gop.None), "")
}

func TestTokenize(t *testing.T) {
	ref := "test"
	timeStamp, _ := time.Parse(time.RFC3339Nano, "2021-08-28T08:36:36.807908+08:00")
	fn := func(string) int { return 10 }
	ch1 := make(chan int)
	ch2 := make(chan string, 3)
	ch3 := make(chan struct{})

	v := []interface{}{
		nil,
		[]int{},
		[]interface{}{true, false, uintptr(0x17), float32(100.121111133)},
		true, 10, int8(2), int32(100),
		float64(100.121111133),
		complex64(1 + 2i), complex128(1 + 2i),
		[3]int{1, 2},
		ch1,
		ch2,
		ch3,
		fn,
		map[interface{}]interface{}{
			`"test"`: 10,
			"a":      1,
			&ref:     1,
		},
		unsafe.Pointer(&ref),
		struct {
			Int int
			str string
			M   map[int]int
		}{10, "ok", map[int]int{1: 0x20}},
		[]byte("aa\xe2"),
		[]byte("bytes\n\tbytes"),
		[]byte("long long long long string"),
		byte('a'),
		byte(1),
		'天',
		"long long long long string",
		"\ntest",
		"\t\n`",
		&ref,
		(*struct{ Int int })(nil),
		&struct{ Int int }{},
		&map[int]int{1: 2, 3: 4},
		&[]int{1, 2},
		&[2]int{1, 2},
		&[]byte{1, 2},
		timeStamp,
		time.Hour,
		`{"a": 1}`,
		[]byte(`{"a": 1}`),
	}

	check := func(out string, file string) {
		t.Helper()

		tpl, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		expected := render(string(tpl), map[string]interface{}{
			"ch1": fmt.Sprintf("0x%x", reflect.ValueOf(ch1).Pointer()),
			"ch2": fmt.Sprintf("0x%x", reflect.ValueOf(ch2).Pointer()),
			"ch3": fmt.Sprintf("0x%x", reflect.ValueOf(ch3).Pointer()),
			"fn":  fmt.Sprintf("0x%x", reflect.ValueOf(fn).Pointer()),
			"ptr": fmt.Sprintf("%v", &ref),
			"ref": fmt.Sprintf("0x%x", reflect.ValueOf(&ref).Pointer()),
		})

		if out != expected {
			t.Log("check failed")
			t.Fail()
		}
	}

	out := gop.StripANSI(gop.F(v))

	{
		b, err := os.ReadFile(filepath.Join("fixtures", "compile_check.go.tmpl"))
		if err != nil {
			t.Fatal(err)
		}
		code := fmt.Sprintf(string(b), out)
		f := filepath.Join("tmp", randStr(8), "main.go")
		err = os.MkdirAll(filepath.Dir(f), 0755)
		if err != nil {
			t.Fatal(err)
		}
		err = os.WriteFile(f, []byte(code), 0644)
		if err != nil {
			t.Fatal(err)
		}
		b, err = exec.Command("go", "run", f).CombinedOutput()
		if err != nil {
			t.Fatal(string(b))
		}
	}

	check(out, filepath.Join("fixtures", "expected.tmpl"))

	out = gop.VisualizeANSI(gop.F(v))
	check(out, filepath.Join("fixtures", "expected_with_color.tmpl"))
}

func TestRef(t *testing.T) {
	a := [2][]int{{1}}
	a[1] = a[0]

	eq(t, gop.Plain(a), `[2][]int{
    []int/* len=1 cap=1 */{
        1,
    },
    []int/* len=1 cap=1 */{
        1,
    },
}`)
}

type A struct {
	Int int
	B   *B
}

type B struct {
	s string
	a *A
}

func TestCircularRef(t *testing.T) {
	a := A{Int: 10}
	b := B{"test", &a}
	a.B = &b

	eq(t, gop.StripANSI(gop.F(a)), `gop_test.A{
    Int: 10,
    B: &gop_test.B{
        s: "test",
        a: &gop_test.A{
            Int: 10,
            B: gop.Circular("B").(*gop_test.B),
        },
    },
}`)
}

func TestCircularNilRef(t *testing.T) {
	arr := []A{{}, {}}

	eq(t, gop.StripANSI(gop.F(arr)), `[]gop_test.A/* len=2 cap=2 */{
    gop_test.A{
        Int: 0,
        B: (*gop_test.B)(nil),
    },
    gop_test.A{
        Int: 0,
        B: (*gop_test.B)(nil),
    },
}`)
}

func TestCircularMap(t *testing.T) {
	a := map[int]interface{}{}
	a[0] = a

	ts := gop.Tokenize(a)

	eq(t, gop.Format(ts, gop.ThemeNone), `map[int]interface {}{
    0: gop.Circular().(map[int]interface {}),
}`)
}

func TestCircularSlice(t *testing.T) {
	a := [][]interface{}{{nil}, {nil}}
	a[0][0] = a[1]
	a[1][0] = a[0][0]

	ts := gop.Tokenize(a)

	eq(t, gop.Format(ts, gop.ThemeNone), `[][]interface {}/* len=2 cap=2 */{
    []interface {}/* len=1 cap=1 */{
        []interface {}/* len=1 cap=1 */{
            gop.Circular(0, 0).([]interface {}),
        },
    },
    []interface {}/* len=1 cap=1 */{
        gop.Circular(1).([]interface {}),
    },
}`)
}

func TestCircularMapKey(t *testing.T) {
	a := map[interface{}]interface{}{}
	b := map[interface{}]interface{}{}
	a[&b] = b
	b[&a] = a

	ts := gop.Tokenize(b)

	eq(t, gop.Format(ts, gop.ThemeNone), render(`map[interface {}]interface {}{
    (interface {})(nil)/* {{.a}} */: map[interface {}]interface {}{
        (interface {})(nil)/* {{.b}} */: gop.Circular().(map[interface {}]interface {}),
    },
}`, map[string]interface{}{
		"a": fmt.Sprintf("0x%x", reflect.ValueOf(&a).Pointer()),
		"b": fmt.Sprintf("0x%x", reflect.ValueOf(&b).Pointer()),
	}))
}

func TestPlain(t *testing.T) {
	eq(t, gop.Plain(10), "10")
}

func TestPlainMinify(t *testing.T) {
	a := map[int]interface{}{
		1: "a",
	}
	b := map[int]interface{}{}
	pa := gop.Plain(a)
	pb := gop.Plain(b)
	eq(t, pa, "map[int]interface {}{\n    1: \"a\",\n}")
	eq(t, pb, "map[int]interface {}{}")
}

func TestP(t *testing.T) {
	gop.Stdout = io.Discard
	_ = gop.P("test")
	gop.Stdout = os.Stdout
}

func TestConvertors(t *testing.T) {
	eq(t, gop.Circular(""), nil)

	s := randStr(8)
	eq(t, *gop.Ptr(s).(*string), s)

	bs := base64.StdEncoding.EncodeToString([]byte(s))

	eq(t, string(gop.Base64(bs)), string([]byte(s)))
	now := time.Now()
	eq(t, gop.Time(now.Format(time.RFC3339Nano), 1234).Unix(), now.Unix())
	eq(t, gop.Duration("10m"), 10*time.Minute)

	eq(t, gop.JSONStr(nil, "[1, 2]"), "[1, 2]")
	eq(t, string(gop.JSONBytes(nil, "[1, 2]")), string([]byte("[1, 2]")))
}

func TestGetPrivateFieldErr(t *testing.T) {
	assertPanic(t, func() {
		gop.GetPrivateField(reflect.ValueOf(1), 0)
	})
	assertPanic(t, func() {
		gop.GetPrivateFieldByName(reflect.ValueOf(1), "test")
	})
}

func TestTypeName(t *testing.T) {
	type f float64
	type i int
	type c complex128
	type b byte

	eq(t, gop.Plain(f(1)), "gop_test.f(1.0)")
	eq(t, gop.Plain(i(1)), "gop_test.i(1)")
	eq(t, gop.Plain(c(1)), "gop_test.c(1+0i)")
	eq(t, gop.Plain(b('a')), "gop_test.b(97)")
}

func TestSliceCapNotEqual(t *testing.T) {
	x := gop.Plain(make([]int, 3, 10))
	y := gop.Plain(make([]int, 3))

	neq(t, x, y)
}

func TestFixNestedStyle(t *testing.T) {
	s := gop.S(" 0 "+gop.S(" 1 "+
		gop.S(" 2 "+
			gop.S(" 3 ", gop.Cyan)+
			" 4 ", gop.Blue)+
		" 5 ", gop.Red)+" 6 ", gop.BgRed)
	out := gop.VisualizeANSI(gop.FixNestedStyle(s))
	eq(t, out, `<41> 0 <31> 1 <39><34> 2 <39><36> 3 <39><34> 4 <39><31> 5 <39> 6 <49>`)

	gop.FixNestedStyle("test")
}

func TestStripANSI(t *testing.T) {
	eq(t, gop.StripANSI(gop.S("test", gop.Red)), "test")
}

func TestTheme(t *testing.T) {
	eq(t, gop.ThemeDefault(gop.Error)[0], gop.Underline)
}

func TestNil(t *testing.T) {
	eq(t, gop.Plain(map[string]string(nil)), "map[string]string(nil)")
	eq(t, gop.Plain(chan int(nil)), "(chan int)(nil)")
	eq(t, gop.Plain([]string(nil)), "[]string(nil)")
	eq(t, gop.Plain((func())(nil)), "(func())(nil)")
	eq(t, gop.Plain((*struct{})(nil)), "(*struct {})(nil)")
}
