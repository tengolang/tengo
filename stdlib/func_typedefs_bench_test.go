package stdlib

import (
	"strconv"
	"testing"

	"github.com/ganehag/tengo/v3"
)

func benchStrings(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "s" + strconv.Itoa(i)
	}
	return out
}

func benchInts(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func BenchmarkFuncARSs(b *testing.B) {
	fn := FuncARSs(func() []string { return benchStrings(1000) })
	for i := 0; i < b.N; i++ {
		obj, err := fn()
		if err != nil {
			b.Fatal(err)
		}
		if len(obj.(*tengo.Array).Value) != 1000 {
			b.Fatal("wrong length")
		}
	}
}

func BenchmarkFuncASRSs(b *testing.B) {
	fn := FuncASRSs(func(string) []string { return benchStrings(1000) })
	for i := 0; i < b.N; i++ {
		obj, err := fn(&tengo.String{Value: "x"})
		if err != nil {
			b.Fatal(err)
		}
		if len(obj.(*tengo.Array).Value) != 1000 {
			b.Fatal("wrong length")
		}
	}
}

func BenchmarkFuncASSRSs(b *testing.B) {
	fn := FuncASSRSs(func(string, string) []string { return benchStrings(1000) })
	for i := 0; i < b.N; i++ {
		obj, err := fn(&tengo.String{Value: "x"}, &tengo.String{Value: "y"})
		if err != nil {
			b.Fatal(err)
		}
		if len(obj.(*tengo.Array).Value) != 1000 {
			b.Fatal("wrong length")
		}
	}
}

func BenchmarkFuncASSIRSs(b *testing.B) {
	fn := FuncASSIRSs(func(string, string, int) []string { return benchStrings(1000) })
	for i := 0; i < b.N; i++ {
		obj, err := fn(&tengo.String{Value: "x"}, &tengo.String{Value: "y"}, tengo.Int{Value: 1})
		if err != nil {
			b.Fatal(err)
		}
		if len(obj.(*tengo.Array).Value) != 1000 {
			b.Fatal("wrong length")
		}
	}
}

func BenchmarkFuncAIRSsE(b *testing.B) {
	fn := FuncAIRSsE(func(int) ([]string, error) { return benchStrings(1000), nil })
	for i := 0; i < b.N; i++ {
		obj, err := fn(tengo.Int{Value: 1})
		if err != nil {
			b.Fatal(err)
		}
		if len(obj.(*tengo.Array).Value) != 1000 {
			b.Fatal("wrong length")
		}
	}
}

func BenchmarkFuncARIsE(b *testing.B) {
	fn := FuncARIsE(func() ([]int, error) { return benchInts(1000), nil })
	for i := 0; i < b.N; i++ {
		obj, err := fn()
		if err != nil {
			b.Fatal(err)
		}
		if len(obj.(*tengo.Array).Value) != 1000 {
			b.Fatal("wrong length")
		}
	}
}

func BenchmarkFuncAIRIs(b *testing.B) {
	fn := FuncAIRIs(func(int) []int { return benchInts(1000) })
	for i := 0; i < b.N; i++ {
		obj, err := fn(tengo.Int{Value: 1})
		if err != nil {
			b.Fatal(err)
		}
		if len(obj.(*tengo.Array).Value) != 1000 {
			b.Fatal("wrong length")
		}
	}
}
