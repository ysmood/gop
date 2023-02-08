package gop_test

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/ysmood/gop"
	"github.com/ysmood/got"
	"github.com/ysmood/got/lib/diff"
)

func TestStyle(t *testing.T) {
	g := got.T(t)
	s := gop.Style{Set: "<s>", Unset: "</s>"}

	g.Eq(gop.S("test", s), "<s>test</s>")
	g.Eq(gop.S("", s), "<s></s>")
	g.Eq(gop.S("", gop.None), "")
}

func TestTokenize(t *testing.T) {
	g := got.T(t)
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

	check := func(out string, tpl string) {
		g.Helper()

		expected := g.Render(tpl, map[string]interface{}{
			"ch1": fmt.Sprintf("0x%x", reflect.ValueOf(ch1).Pointer()),
			"ch2": fmt.Sprintf("0x%x", reflect.ValueOf(ch2).Pointer()),
			"ch3": fmt.Sprintf("0x%x", reflect.ValueOf(ch3).Pointer()),
			"fn":  fmt.Sprintf("0x%x", reflect.ValueOf(fn).Pointer()),
			"ptr": fmt.Sprintf("%v", &ref),
			"ref": fmt.Sprintf("0x%x", reflect.ValueOf(&ref).Pointer()),
		}).String()

		if out != expected {
			g.Fail()
			g.Log(diff.Diff(out, expected))
		}
	}

	out := gop.StripANSI(gop.F(v))

	{
		code := fmt.Sprintf(g.Read(filepath.Join("fixtures", "compile_check.go.tmpl")).String(), out)
		f := filepath.Join("tmp", g.RandStr(8), "main.go")
		g.WriteFile(f, code)
		b, err := exec.Command("go", "run", f).CombinedOutput()
		if err != nil {
			g.Error(string(b))
		}
	}

	check(out, filepath.Join("fixtures", "expected.tmpl"))

	out = gop.VisualizeANSI(gop.F(v))
	check(out, filepath.Join("fixtures", "expected_with_color.tmpl"))
}

func TestRef(t *testing.T) {
	g := got.T(t)
	a := [2][]int{{1}}
	a[1] = a[0]

	g.Eq(gop.Plain(a), `[2][]int{
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
	g := got.T(t)
	a := A{Int: 10}
	b := B{"test", &a}
	a.B = &b

	g.Eq(gop.StripANSI(gop.F(a)), `gop_test.A{
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

	got.T(t).Eq(gop.StripANSI(gop.F(arr)), `[]gop_test.A/* len=2 cap=2 */{
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
	g := got.T(t)
	a := map[int]interface{}{}
	a[0] = a

	ts := gop.Tokenize(a)

	g.Eq(gop.Format(ts, gop.ThemeNone), `map[int]interface {}{
    0: gop.Circular().(map[int]interface {}),
}`)
}

func TestCircularSlice(t *testing.T) {
	g := got.New(t)
	a := [][]interface{}{{nil}, {nil}}
	a[0][0] = a[1]
	a[1][0] = a[0][0]

	ts := gop.Tokenize(a)

	g.Eq(gop.Format(ts, gop.ThemeNone), `[][]interface {}/* len=2 cap=2 */{
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
	g := got.New(t)

	a := map[interface{}]interface{}{}
	b := map[interface{}]interface{}{}
	a[&b] = b
	b[&a] = a

	ts := gop.Tokenize(b)

	g.Eq(gop.Format(ts, gop.ThemeNone), g.Render(`map[interface {}]interface {}{
    (interface {})(nil)/* {{.a}} */: map[interface {}]interface {}{
        (interface {})(nil)/* {{.b}} */: gop.Circular().(map[interface {}]interface {}),
    },
}`, map[string]interface{}{
		"a": fmt.Sprintf("0x%x", reflect.ValueOf(&a).Pointer()),
		"b": fmt.Sprintf("0x%x", reflect.ValueOf(&b).Pointer()),
	}).String())
}

func TestPlain(t *testing.T) {
	g := got.T(t)
	g.Eq(gop.Plain(10), "10")
}

func TestPlainMinify(t *testing.T) {
	g := got.T(t)
	a := map[int]interface{}{
		1: "a",
	}
	b := map[int]interface{}{}
	pa := gop.Plain(a)
	pb := gop.Plain(b)
	g.Eq(pa, "map[int]interface {}{\n    1: \"a\",\n}")
	g.Eq(pb, "map[int]interface {}{}")
}

func TestP(t *testing.T) {
	gop.Stdout = ioutil.Discard
	_ = gop.P("test")
	gop.Stdout = os.Stdout
}

func TestConvertors(t *testing.T) {
	g := got.T(t)
	g.Nil(gop.Circular(""))

	s := g.RandStr(8)
	g.Eq(gop.Ptr(s).(*string), &s)

	bs := base64.StdEncoding.EncodeToString([]byte(s))

	g.Eq(gop.Base64(bs), []byte(s))
	now := time.Now()
	g.Eq(gop.Time(now.Format(time.RFC3339Nano), 1234), now)
	g.Eq(gop.Duration("10m"), 10*time.Minute)

	g.Eq(gop.JSONStr(nil, "[1, 2]"), "[1, 2]")
	g.Eq(gop.JSONBytes(nil, "[1, 2]"), []byte("[1, 2]"))
}

func TestGetPrivateFieldErr(t *testing.T) {
	g := got.T(t)
	g.Panic(func() {
		gop.GetPrivateField(reflect.ValueOf(1), 0)
	})
	g.Panic(func() {
		gop.GetPrivateFieldByName(reflect.ValueOf(1), "test")
	})
}

func TestTypeName(t *testing.T) {
	g := got.T(t)

	type f float64
	type i int
	type c complex128
	type b byte

	g.Eq(gop.Plain(f(1)), "gop_test.f(1.0)")
	g.Eq(gop.Plain(i(1)), "gop_test.i(1)")
	g.Eq(gop.Plain(c(1)), "gop_test.c(1+0i)")
	g.Eq(gop.Plain(b('a')), "gop_test.b(97)")
}

func TestSliceCapNotEqual(t *testing.T) {
	g := got.T(t)

	x := gop.Plain(make([]int, 3, 10))
	y := gop.Plain(make([]int, 3))

	g.Desc("we should show the diff of cap").Neq(x, y)
}

func TestFixNestedStyle(t *testing.T) {
	g := got.T(t)

	s := gop.S(" 0 "+gop.S(" 1 "+
		gop.S(" 2 "+
			gop.S(" 3 ", gop.Cyan)+
			" 4 ", gop.Blue)+
		" 5 ", gop.Red)+" 6 ", gop.BgRed)
	fmt.Println(gop.VisualizeANSI(s))
	out := gop.VisualizeANSI(gop.FixNestedStyle(s))
	g.Eq(out, `<41> 0 <31> 1 <39><34> 2 <39><36> 3 <39><34> 4 <39><31> 5 <39> 6 <49>`)

	gop.FixNestedStyle("test")
}

func TestStripANSI(t *testing.T) {
	g := got.T(t)
	g.Eq(gop.StripANSI(gop.S("test", gop.Red)), "test")
}

func TestTheme(t *testing.T) {
	g := got.T(t)
	g.Eq(gop.ThemeDefault(gop.Error), []gop.Style{gop.Underline, gop.Red})
}
