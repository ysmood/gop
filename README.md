# Go Pretty Print Value

Make a random Go value human readable. The output format uses valid golang syntax, so you don't have to learn any new knowledge to understand the output.

## Features

- Uses valid golang syntax to print the data
- Make rune, []byte, time, etc. data human readable
- Color output with customizable theme
- Stable map output with sorted by keys
- Auto split multiline large string block
- Prints the path of circular reference
- Auto format inline json string
- Low-level API to extend the lib

## Usage

Usually, you only need to use `gop.P` function:

```go
package main

import (
    "time"

    "github.com/ysmood/got/lib/gop"
)

func main() {
    val := map[string]interface{}{
        "bool":   true,
        "number": 1 + 1i,
        "bytes":  []byte{97, 98, 99},
        "lines":  "multiline string\nline two",
        "slice":  []interface{}{1, 2},
        "time":   time.Now(),
        "chan":   make(chan int, 1),
        "struct": struct{ test int32 }{
            test: 13,
        },
        "json": `{"a"   : 1}`,
        "func": func(int) int { return 0 },
    }
    val["slice"].([]interface{})[1] = val["slice"]

    _ = gop.P(val)
}
```

The output will be something like:

```go
// 2023-10-07T18:19:57.517309+08:00 example/main.go:27 (main.main)
map[string]interface {}{
    "bool": true,
    "bytes": []byte("abc"),
    "chan": make(chan int, 1)/* 0x1400008c070 */,
    "func": (func(int) int)(nil)/* 0x1025a5460 */,
    "json": gop.JSONStr(map[string]interface {}{
        "a": 1.0,
    }, `{"a"   : 1}`),
    "lines": `multiline string
line two`,
    "number": 1+1i,
    "slice": []interface {}{
        1,
        gop.Circular("slice").([]interface {}),
    },
    "struct": struct { test int32 }{
        test: int32(13),
    },
    "time": gop.Time("2023-10-07T18:19:57.516984+08:00", 3081584),
}
```
