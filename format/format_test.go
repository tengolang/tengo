package format_test

import (
	"testing"

	"github.com/tengolang/tengo/v3/format"
	"github.com/tengolang/tengo/v3/require"
)

func TestFormat(t *testing.T) {
	cases := []struct {
		name string
		in   string
		out  string
	}{
		{
			name: "assign",
			in:   `x:=1`,
			out:  "x := 1\n",
		},
		{
			name: "binary no extra parens",
			in:   `x:=a+b*c`,
			out:  "x := a + b * c\n",
		},
		{
			name: "binary lower prec needs parens",
			in:   `x:=(a+b)*c`,
			out:  "x := (a + b) * c\n",
		},
		{
			name: "if",
			in:   `if x>0{y:=1}`,
			out:  "if x > 0 {\n\ty := 1\n}\n",
		},
		{
			name: "if-else",
			in:   `if x>0{y:=1}else{y:=2}`,
			out:  "if x > 0 {\n\ty := 1\n} else {\n\ty := 2\n}\n",
		},
		{
			name: "if-else-if",
			in:   `if x>0{y:=1}else if x<0{y:=-1}else{y:=0}`,
			out:  "if x > 0 {\n\ty := 1\n} else if x < 0 {\n\ty := -1\n} else {\n\ty := 0\n}\n",
		},
		{
			name: "for while",
			in:   `for x>0{x--}`,
			out:  "for x > 0 {\n\tx--\n}\n",
		},
		{
			name: "for c-style",
			in:   `for i:=0;i<10;i++{x+=i}`,
			out:  "for i := 0; i < 10; i++ {\n\tx += i\n}\n",
		},
		{
			name: "for-in",
			in:   `for k,v in m{x+=v}`,
			out:  "for k, v in m {\n\tx += v\n}\n",
		},
		{
			name: "func literal",
			in:   `f:=func(a,b){return a+b}`,
			out:  "f := func(a, b) {\n\treturn a + b\n}\n",
		},
		{
			name: "varargs",
			in:   `f:=func(a,...b){return b}`,
			out:  "f := func(a, ...b) {\n\treturn b\n}\n",
		},
		{
			name: "map literal",
			in:   `m:={a:1,b:2}`,
			out:  "m := {\n\ta: 1,\n\tb: 2,\n}\n",
		},
		{
			name: "empty map",
			in:   `m:={}`,
			out:  "m := {}\n",
		},
		{
			name: "array literal",
			in:   `a:=[1,2,3]`,
			out:  "a := [1, 2, 3]\n",
		},
		{
			name: "switch",
			in:   `switch x{case 1:y:=1 case 2:y:=2 default:y:=0}`,
			out:  "switch x {\ncase 1:\n\ty := 1\ncase 2:\n\ty := 2\ndefault:\n\ty := 0\n}\n",
		},
		{
			name: "import",
			in:   `fmt:=import("fmt")`,
			out:  "fmt := import(\"fmt\")\n",
		},
		{
			name: "return multi",
			in:   `return a,b`,
			out:  "return a, b\n",
		},
		{
			name: "line comment",
			in:   "// header\nx := 1",
			out:  "// header\nx := 1\n",
		},
		{
			name: "trailing comment",
			in:   "x := 1 // set x",
			out:  "x := 1 // set x\n",
		},
		{
			name: "blank line preserved",
			in:   "x := 1\n\ny := 2",
			out:  "x := 1\n\ny := 2\n",
		},
		{
			name: "idempotent",
			in:   "x := 1\n",
			out:  "x := 1\n",
		},
		{
			name: "selector",
			in:   `x:=a.b.c`,
			out:  "x := a.b.c\n",
		},
		{
			name: "index",
			in:   `x:=a[0]`,
			out:  "x := a[0]\n",
		},
		{
			name: "slice",
			in:   `x:=a[1:3]`,
			out:  "x := a[1:3]\n",
		},
		{
			name: "ternary",
			in:   `x:=a?b:c`,
			out:  "x := a ? b : c\n",
		},
		{
			name: "error expr",
			in:   `x:=error("oops")`,
			out:  "x := error(\"oops\")\n",
		},
		{
			name: "method call",
			in:   `x:=obj::method(1,2)`,
			out:  "x := obj::method(1, 2)\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := format.Format([]byte(tc.in))
			require.NoError(t, err)
			require.Equal(t, tc.out, string(got))
		})
	}
}

func TestFormatIdempotent(t *testing.T) {
	src := `fmt := import("fmt")

// add returns a + b
add := func(a, b) {
	return a + b
}

x := add(1, 2)
fmt.println(x)
`
	got, err := format.Format([]byte(src))
	require.NoError(t, err)
	require.Equal(t, src, string(got))

	// second pass must be identical
	got2, err := format.Format(got)
	require.NoError(t, err)
	require.Equal(t, string(got), string(got2))
}

func TestFormatParseError(t *testing.T) {
	_, err := format.Format([]byte(`x := (`))
	require.Error(t, err)
}
