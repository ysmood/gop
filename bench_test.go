package gop_test

import (
	"testing"
	"time"
	"unsafe"

	"github.com/ysmood/gop"
)

func benchValue() interface{} {
	ref := "test"
	timeStamp, _ := time.Parse(time.RFC3339Nano, "2021-08-28T08:36:36.807908+08:00")
	fn := func(string) int { return 10 }
	ch1 := make(chan int)
	ch2 := make(chan string, 3)
	ch3 := make(chan struct{})

	return []interface{}{
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
}

func BenchmarkTokenize(b *testing.B) {
	v := benchValue()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = gop.Tokenize(v)
	}
}

func BenchmarkFormatDefault(b *testing.B) {
	v := benchValue()
	ts := gop.Tokenize(v)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = gop.Format(ts, gop.ThemeDefault)
	}
}

func BenchmarkFormatNone(b *testing.B) {
	v := benchValue()
	ts := gop.Tokenize(v)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = gop.Format(ts, gop.ThemeNone)
	}
}

func BenchmarkF(b *testing.B) {
	v := benchValue()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = gop.F(v)
	}
}

func BenchmarkPlain(b *testing.B) {
	v := benchValue()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = gop.Plain(v)
	}
}
