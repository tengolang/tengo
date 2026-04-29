package tengo_test

// bytecode_format_test.go — intensive tests for the section-based binary
// bytecode format introduced in format version 0x02.
//
// Coverage:
//  - Header field detection helpers (IsBytecodeData, BytecodeDataVersion, BytecodeDataKind)
//  - All object type tags (Undefined, Int, Float, Bool, Char, String, Bytes,
//    Array, ImmutableArray, Map, ImmutableMap, CompiledFunction, ModuleRef,
//    Error, Time) including edge-case values
//  - Nested and deeply nested object structures
//  - CompiledFunction encoding (instructions, source map, varargs, NumLocals)
//  - ModuleRef: resolves live stdlib module at decode time, errors clearly
//    when module is absent
//  - DSET (source file set) round-trip with multiple files and line tables
//  - Section ordering: unknown sections are skipped without error
//  - Error messages: no-magic, wrong version, missing MAIN, unknown tag
//  - Deterministic encoding: maps always sort keys
//  - EncodeModule vs Encode kind byte
//  - Full script compile → encode → decode → run integration
//  - Pre-compiled module import workflow

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/tengolang/tengo/v3"
	"github.com/tengolang/tengo/v3/parser"
	"github.com/tengolang/tengo/v3/require"
	"github.com/tengolang/tengo/v3/stdlib"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// roundTrip encodes b and decodes it back, using modules for ModuleRef
// resolution. Returns the decoded Bytecode.
func roundTrip(t *testing.T, b *tengo.Bytecode, modules *tengo.ModuleMap) *tengo.Bytecode {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, b.Encode(&buf))
	got := &tengo.Bytecode{}
	require.NoError(t, got.Decode(bytes.NewReader(buf.Bytes()), modules))
	return got
}

// roundTripModule encodes b as a module and decodes it back.
func roundTripModule(t *testing.T, b *tengo.Bytecode, modules *tengo.ModuleMap) *tengo.Bytecode {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, b.EncodeModule(&buf))
	got := &tengo.Bytecode{}
	require.NoError(t, got.Decode(bytes.NewReader(buf.Bytes()), modules))
	return got
}

// minBytecode wraps a constant slice in a minimal Bytecode with an empty
// main function. Used for object type round-trip tests.
func minBytecode(constants []tengo.Object) *tengo.Bytecode {
	return &tengo.Bytecode{
		FileSet:      parser.NewFileSet(),
		MainFunction: &tengo.CompiledFunction{Instructions: []byte{}},
		Constants:    constants,
	}
}

// runBytecode runs decoded bytecode and returns the globals slice so
// integration tests can inspect variable values.
func runBytecode(t *testing.T, bc *tengo.Bytecode) []tengo.Object {
	t.Helper()
	globals := make([]tengo.Object, tengo.GlobalsSize)
	vm := tengo.NewVM(bc, globals, -1)
	require.NoError(t, vm.Run())
	return globals
}

// ---------------------------------------------------------------------------
// 1. Header helpers
// ---------------------------------------------------------------------------

func TestBytecodeFormat_IsBytecodeData(t *testing.T) {
	t.Run("valid magic", func(t *testing.T) {
		var buf bytes.Buffer
		bc := minBytecode(nil)
		require.NoError(t, bc.Encode(&buf))
		require.True(t, tengo.IsBytecodeData(buf.Bytes()))
	})

	t.Run("empty slice", func(t *testing.T) {
		require.False(t, tengo.IsBytecodeData(nil))
		require.False(t, tengo.IsBytecodeData([]byte{}))
	})

	t.Run("too short", func(t *testing.T) {
		require.False(t, tengo.IsBytecodeData([]byte{0x1B, 'T', 'n'}))
	})

	t.Run("tengo source code", func(t *testing.T) {
		require.False(t, tengo.IsBytecodeData([]byte(`a := 1 + 2`)))
	})

	t.Run("wrong first byte", func(t *testing.T) {
		require.False(t, tengo.IsBytecodeData([]byte{0x00, 'T', 'n', 'g'}))
	})
}

func TestBytecodeFormat_BytecodeDataVersion(t *testing.T) {
	var buf bytes.Buffer
	bc := minBytecode(nil)
	require.NoError(t, bc.Encode(&buf))
	data := buf.Bytes()

	require.Equal(t, int(tengo.BytecodeFormatVersion), int(tengo.BytecodeDataVersion(data)))
	require.Equal(t, 0, int(tengo.BytecodeDataVersion(nil)))
	require.Equal(t, 0, int(tengo.BytecodeDataVersion([]byte("not bytecode"))))
}

func TestBytecodeFormat_BytecodeDataKind(t *testing.T) {
	t.Run("script", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, minBytecode(nil).Encode(&buf))
		require.Equal(t, int(tengo.BytecodeKindScript), int(tengo.BytecodeDataKind(buf.Bytes())))
	})

	t.Run("module", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, minBytecode(nil).EncodeModule(&buf))
		require.Equal(t, int(tengo.BytecodeKindModule), int(tengo.BytecodeDataKind(buf.Bytes())))
	})

	t.Run("no header", func(t *testing.T) {
		require.Equal(t, int(tengo.BytecodeKindScript), int(tengo.BytecodeDataKind(nil)))
	})
}

// ---------------------------------------------------------------------------
// 2. Decode error cases
// ---------------------------------------------------------------------------

func TestBytecodeFormat_DecodeErrors(t *testing.T) {
	t.Run("no magic header", func(t *testing.T) {
		err := (&tengo.Bytecode{}).Decode(
			bytes.NewReader([]byte(`a := 1`)), nil)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "does not appear"))
	})

	t.Run("version 0x01 (old gob format)", func(t *testing.T) {
		// Craft a header with version 0x01 manually.
		header := []byte{0x1B, 'T', 'n', 'g', 0x01, 0x01, 0x00, 0x00}
		err := (&tengo.Bytecode{}).Decode(bytes.NewReader(header), nil)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "0x01"))
		require.True(t, strings.Contains(err.Error(), "recompile"))
	})

	t.Run("unknown version", func(t *testing.T) {
		header := []byte{0x1B, 'T', 'n', 'g', 0xFF, 0x01, 0x00, 0x00}
		err := (&tengo.Bytecode{}).Decode(bytes.NewReader(header), nil)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "0xff"))
	})

	t.Run("valid header but no MAIN section", func(t *testing.T) {
		// Write a valid header + CONS section only, omit MAIN.
		var buf bytes.Buffer
		bc := minBytecode(nil)
		require.NoError(t, bc.Encode(&buf))
		// Replace MAIN section with an unknown tag so it is skipped.
		data := buf.Bytes()
		// CONS header is at byte 8: "CONS" + 4-byte length.
		// MAIN header follows: find "MAIN" and replace with "XXXX".
		for i := 0; i < len(data)-3; i++ {
			if data[i] == 'M' && data[i+1] == 'A' && data[i+2] == 'I' && data[i+3] == 'N' {
				data[i], data[i+1], data[i+2], data[i+3] = 'X', 'X', 'X', 'X'
				break
			}
		}
		err := (&tengo.Bytecode{}).Decode(bytes.NewReader(data), nil)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "MAIN"))
	})

	t.Run("truncated file", func(t *testing.T) {
		var buf bytes.Buffer
		bc := minBytecode(nil)
		require.NoError(t, bc.Encode(&buf))
		// Feed only the first 12 bytes — header + start of CONS section header.
		err := (&tengo.Bytecode{}).Decode(
			bytes.NewReader(buf.Bytes()[:12]), nil)
		require.Error(t, err)
	})

	t.Run("unknown object tag in constants", func(t *testing.T) {
		// Create valid bytecode, then corrupt the first constant's type tag.
		bc := minBytecode([]tengo.Object{tengo.Int{Value: 42}})
		var buf bytes.Buffer
		require.NoError(t, bc.Encode(&buf))
		data := buf.Bytes()
		// The CONS data starts after:
		//   8 (header) + 4 (CONS tag) + 4 (CONS length) + 4 (count) = 20 bytes
		// The first object tag is at byte 20.
		data[20] = 0xFF // invalid tag
		err := (&tengo.Bytecode{}).Decode(bytes.NewReader(data), nil)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "0xff"))
	})
}

// ---------------------------------------------------------------------------
// 3. Object type round-trips
// ---------------------------------------------------------------------------

func TestBytecodeFormat_ObjectTypes(t *testing.T) {
	check := func(t *testing.T, objs ...tengo.Object) {
		t.Helper()
		bc := minBytecode(objs)
		got := roundTrip(t, bc, nil)
		require.Equal(t, bc.Constants, got.Constants)
	}

	t.Run("Undefined", func(t *testing.T) {
		check(t, tengo.UndefinedValue)
		// Verify it returns the singleton pointer.
		bc := minBytecode([]tengo.Object{tengo.UndefinedValue})
		got := roundTrip(t, bc, nil)
		if got.Constants[0] != tengo.UndefinedValue {
			t.Error("decoded Undefined is not the global singleton")
		}
	})

	t.Run("Bool singletons", func(t *testing.T) {
		check(t, tengo.TrueValue, tengo.FalseValue)
		bc := minBytecode([]tengo.Object{tengo.TrueValue, tengo.FalseValue})
		got := roundTrip(t, bc, nil)
		if got.Constants[0] != tengo.TrueValue {
			t.Error("decoded true is not the global TrueValue singleton")
		}
		if got.Constants[1] != tengo.FalseValue {
			t.Error("decoded false is not the global FalseValue singleton")
		}
	})

	t.Run("Int", func(t *testing.T) {
		check(t,
			tengo.Int{Value: 0},
			tengo.Int{Value: 1},
			tengo.Int{Value: -1},
			tengo.Int{Value: math.MaxInt64},
			tengo.Int{Value: math.MinInt64},
		)
	})

	t.Run("Float", func(t *testing.T) {
		check(t,
			tengo.Float{Value: 0},
			tengo.Float{Value: 3.14159265358979323846},
			tengo.Float{Value: -2.718281828},
			tengo.Float{Value: math.MaxFloat64},
			tengo.Float{Value: math.SmallestNonzeroFloat64},
			tengo.Float{Value: math.Inf(1)},
			tengo.Float{Value: math.Inf(-1)},
		)
	})

	t.Run("Float NaN round-trips as NaN", func(t *testing.T) {
		bc := minBytecode([]tengo.Object{tengo.Float{Value: math.NaN()}})
		var buf bytes.Buffer
		require.NoError(t, bc.Encode(&buf))
		got := &tengo.Bytecode{}
		require.NoError(t, got.Decode(bytes.NewReader(buf.Bytes()), nil))
		f, ok := got.Constants[0].(tengo.Float)
		require.True(t, ok)
		require.True(t, math.IsNaN(f.Value), "expected NaN, got %v", f.Value)
	})

	t.Run("Char", func(t *testing.T) {
		check(t,
			tengo.Char{Value: 0},
			tengo.Char{Value: 'A'},
			tengo.Char{Value: '日'},
			tengo.Char{Value: math.MaxInt32},
		)
	})

	t.Run("String", func(t *testing.T) {
		check(t,
			&tengo.String{Value: ""},
			&tengo.String{Value: "hello, world"},
			&tengo.String{Value: "日本語テスト"},
			&tengo.String{Value: strings.Repeat("x", 64*1024)}, // 64 KB
		)
	})

	t.Run("Bytes", func(t *testing.T) {
		check(t,
			&tengo.Bytes{Value: []byte{}},
			&tengo.Bytes{Value: []byte{0, 1, 2, 3, 255}},
			&tengo.Bytes{Value: bytes.Repeat([]byte{0xAB}, 1024)},
		)
	})

	t.Run("Array empty", func(t *testing.T) {
		check(t, &tengo.Array{Value: []tengo.Object{}})
	})

	t.Run("Array nested", func(t *testing.T) {
		check(t, &tengo.Array{
			Value: []tengo.Object{
				tengo.Int{Value: 1},
				tengo.TrueValue,
				tengo.UndefinedValue,
				&tengo.Array{
					Value: []tengo.Object{
						&tengo.String{Value: "inner"},
						tengo.Float{Value: 2.5},
					},
				},
			},
		})
	})

	t.Run("ImmutableArray", func(t *testing.T) {
		check(t, &tengo.ImmutableArray{
			Value: []tengo.Object{
				tengo.Int{Value: 10},
				tengo.FalseValue,
			},
		})
	})

	t.Run("Map empty", func(t *testing.T) {
		check(t, &tengo.Map{Value: map[string]tengo.Object{}})
	})

	t.Run("Map all scalar types", func(t *testing.T) {
		check(t, &tengo.Map{
			Value: map[string]tengo.Object{
				"i":  tengo.Int{Value: 99},
				"f":  tengo.Float{Value: 1.5},
				"b":  tengo.TrueValue,
				"c":  tengo.Char{Value: 'Z'},
				"s":  &tengo.String{Value: "val"},
				"bs": &tengo.Bytes{Value: []byte{0x01}},
				"u":  tengo.UndefinedValue,
			},
		})
	})

	t.Run("ImmutableMap non-module", func(t *testing.T) {
		check(t, &tengo.ImmutableMap{
			Value: map[string]tengo.Object{
				"x": tengo.Int{Value: 7},
				"y": tengo.Int{Value: 8},
			},
		})
	})

	t.Run("Error wrapping String", func(t *testing.T) {
		check(t, &tengo.Error{Value: &tengo.String{Value: "something went wrong"}})
	})

	t.Run("Error wrapping Int", func(t *testing.T) {
		check(t, &tengo.Error{Value: tengo.Int{Value: 42}})
	})

	t.Run("Time UTC", func(t *testing.T) {
		check(t, &tengo.Time{Value: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)})
	})

	t.Run("Time with timezone offset", func(t *testing.T) {
		loc := time.FixedZone("CET", 3600)
		check(t, &tengo.Time{Value: time.Date(2026, 6, 15, 9, 30, 0, 0, loc)})
	})

	t.Run("Time strips monotonic clock", func(t *testing.T) {
		// time.Now() includes a monotonic clock reading. After encode/decode
		// the monotonic component must be gone so reflect.DeepEqual holds.
		now := time.Now()
		bc := minBytecode([]tengo.Object{&tengo.Time{Value: now}})
		var buf bytes.Buffer
		require.NoError(t, bc.Encode(&buf))
		got := &tengo.Bytecode{}
		require.NoError(t, got.Decode(bytes.NewReader(buf.Bytes()), nil))
		decoded := got.Constants[0].(*tengo.Time).Value
		// The decoded time should equal the original (ignoring monotonic).
		if !now.Equal(decoded) {
			t.Errorf("time mismatch: original %v, decoded %v", now, decoded)
		}
		// Compare via Round(0) which strips monotonic.
		if !now.Round(0).Equal(decoded) {
			t.Errorf("time mismatch after stripping monotonic: want %v, got %v",
				now.Round(0), decoded)
		}
	})

	t.Run("multiple constants of each type", func(t *testing.T) {
		check(t,
			tengo.Int{Value: 1}, tengo.Int{Value: 2}, tengo.Int{Value: 3},
			tengo.Float{Value: 1.1}, tengo.Float{Value: 2.2},
			tengo.TrueValue, tengo.FalseValue,
			tengo.UndefinedValue,
			&tengo.String{Value: "a"}, &tengo.String{Value: "b"},
			tengo.Char{Value: 'x'},
		)
	})
}

// ---------------------------------------------------------------------------
// 4. CompiledFunction encoding
// ---------------------------------------------------------------------------

func TestBytecodeFormat_CompiledFunction(t *testing.T) {
	t.Run("empty instructions no source map", func(t *testing.T) {
		fn := &tengo.CompiledFunction{
			Instructions:  []byte{},
			NumLocals:     0,
			NumParameters: 0,
			VarArgs:       false,
		}
		bc := minBytecode([]tengo.Object{fn})
		got := roundTrip(t, bc, nil)
		require.Equal(t, bc.Constants, got.Constants)
	})

	t.Run("with instructions and source map", func(t *testing.T) {
		insts := concatInsts(
			tengo.MakeInstruction(parser.OpConstant, 0),
			tengo.MakeInstruction(parser.OpReturn, 1),
		)
		fn := &tengo.CompiledFunction{
			Instructions: insts,
			SourceMap: map[int]parser.Pos{
				0: 10,
				3: 20,
			},
			NumLocals:     3,
			NumParameters: 2,
			VarArgs:       false,
		}
		bc := minBytecode([]tengo.Object{fn})
		got := roundTrip(t, bc, nil)
		require.Equal(t, bc.Constants, got.Constants)
	})

	t.Run("varargs function", func(t *testing.T) {
		fn := &tengo.CompiledFunction{
			Instructions:  tengo.MakeInstruction(parser.OpReturn, 0),
			NumLocals:     1,
			NumParameters: 1,
			VarArgs:       true,
		}
		bc := minBytecode([]tengo.Object{fn})
		got := roundTrip(t, bc, nil)
		decodedFn := got.Constants[0].(*tengo.CompiledFunction)
		require.True(t, decodedFn.VarArgs)
	})

	t.Run("nested CompiledFunctions", func(t *testing.T) {
		inner := &tengo.CompiledFunction{
			Instructions:  tengo.MakeInstruction(parser.OpReturn, 0),
			NumLocals:     1,
			NumParameters: 1,
		}
		outer := &tengo.CompiledFunction{
			Instructions: concatInsts(
				tengo.MakeInstruction(parser.OpConstant, 0),
				tengo.MakeInstruction(parser.OpReturn, 1),
			),
			NumLocals: 2,
		}
		bc := minBytecode([]tengo.Object{inner, outer})
		got := roundTrip(t, bc, nil)
		require.Equal(t, bc.Constants, got.Constants)
	})

	t.Run("large source map", func(t *testing.T) {
		sm := make(map[int]parser.Pos, 200)
		for i := 0; i < 200; i++ {
			sm[i*3] = parser.Pos(i * 10)
		}
		fn := &tengo.CompiledFunction{
			Instructions: bytes.Repeat([]byte{byte(parser.OpPop)}, 200*3),
			SourceMap:    sm,
			NumLocals:    5,
		}
		bc := minBytecode([]tengo.Object{fn})
		got := roundTrip(t, bc, nil)
		require.Equal(t, bc.Constants, got.Constants)
	})
}

// ---------------------------------------------------------------------------
// 5. ModuleRef (stdlib modules)
// ---------------------------------------------------------------------------

func TestBytecodeFormat_ModuleRef(t *testing.T) {
	fmtModule := stdlib.GetModuleMap("fmt")

	t.Run("encode then decode resolves live module", func(t *testing.T) {
		// Compile a script that imports fmt; the bytecode will have an
		// ImmutableMap (with __module_name__="fmt") in constants.
		s := tengo.NewScript([]byte(`fmt := import("fmt")`))
		s.SetImports(fmtModule)
		compiled, err := s.Compile()
		require.NoError(t, err)

		bc := compiled.Bytecode()
		var buf bytes.Buffer
		require.NoError(t, bc.Encode(&buf))

		got := &tengo.Bytecode{}
		require.NoError(t, got.Decode(
			bytes.NewReader(buf.Bytes()), fmtModule))

		// The decoded constant must be an *ImmutableMap for the fmt module.
		var found bool
		for _, c := range got.Constants {
			if m, ok := c.(*tengo.ImmutableMap); ok {
				if _, ok := m.Value["println"]; ok {
					found = true
					break
				}
			}
		}
		require.True(t, found, "decoded fmt module not found in constants")
	})

	t.Run("decode without modules returns error", func(t *testing.T) {
		s := tengo.NewScript([]byte(`fmt := import("fmt")`))
		s.SetImports(fmtModule)
		compiled, err := s.Compile()
		require.NoError(t, err)

		var buf bytes.Buffer
		require.NoError(t, compiled.Bytecode().Encode(&buf))

		err = (&tengo.Bytecode{}).Decode(
			bytes.NewReader(buf.Bytes()), tengo.NewModuleMap())
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), `"fmt"`))
	})
}

// ---------------------------------------------------------------------------
// 6. FileSet (DSET section)
// ---------------------------------------------------------------------------

func TestBytecodeFormat_FileSet(t *testing.T) {
	t.Run("nil FileSet encodes and decodes to empty", func(t *testing.T) {
		bc := &tengo.Bytecode{
			FileSet:      nil,
			MainFunction: &tengo.CompiledFunction{Instructions: []byte{}},
		}
		var buf bytes.Buffer
		require.NoError(t, bc.Encode(&buf))
		got := &tengo.Bytecode{}
		require.NoError(t, got.Decode(bytes.NewReader(buf.Bytes()), nil))
		// A nil FileSet should decode to an empty FileSet, not nil.
		require.NotNil(t, got.FileSet)
	})

	t.Run("single file no extra lines", func(t *testing.T) {
		fs := parser.NewFileSet()
		fs.AddFile("main.tengo", -1, 200)
		bc := &tengo.Bytecode{
			FileSet:      fs,
			MainFunction: &tengo.CompiledFunction{Instructions: []byte{}},
		}
		got := roundTrip(t, bc, nil)
		require.Equal(t, 1, len(got.FileSet.Files))
		require.Equal(t, "main.tengo", got.FileSet.Files[0].Name)
		require.Equal(t, 200, got.FileSet.Files[0].Size)
	})

	t.Run("multiple files with line tables", func(t *testing.T) {
		fs := parser.NewFileSet()
		f1 := fs.AddFile("a.tengo", -1, 100)
		f1.Lines = []int{0, 20, 40, 60, 80}
		f2 := fs.AddFile("b.tengo", -1, 250)
		f2.Lines = []int{0, 50, 100, 150}

		bc := &tengo.Bytecode{
			FileSet:      fs,
			MainFunction: &tengo.CompiledFunction{Instructions: []byte{}},
		}
		got := roundTrip(t, bc, nil)
		require.Equal(t, 2, len(got.FileSet.Files))
		require.Equal(t, f1.Lines, got.FileSet.Files[0].Lines)
		require.Equal(t, f2.Lines, got.FileSet.Files[1].Lines)
		require.Equal(t, f1.Base, got.FileSet.Files[0].Base)
		require.Equal(t, f2.Base, got.FileSet.Files[1].Base)
	})
}

// ---------------------------------------------------------------------------
// 7. Section behaviour
// ---------------------------------------------------------------------------

func TestBytecodeFormat_Sections(t *testing.T) {
	t.Run("unknown sections are skipped", func(t *testing.T) {
		bc := minBytecode([]tengo.Object{tengo.Int{Value: 7}})
		var buf bytes.Buffer
		require.NoError(t, bc.Encode(&buf))

		// Splice an unknown "XUNK" section before MAIN by rebuilding the
		// buffer: header + CONS + XUNK + rest.
		data := buf.Bytes()
		header := data[:8]

		// Find where MAIN starts.
		mainOff := bytes.Index(data[8:], []byte("MAIN")) + 8

		cons := data[8:mainOff]
		rest := data[mainOff:]

		// Build a fake XUNK section: tag + 4-byte length 4 + 4 bytes payload.
		xunk := append([]byte("XUNK"), 0, 0, 0, 4, 0xDE, 0xAD, 0xBE, 0xEF)

		// Use a fresh buffer to avoid aliasing the original data slice.
		var pb bytes.Buffer
		pb.Write(header)
		pb.Write(cons)
		pb.Write(xunk)
		pb.Write(rest)
		patched := pb.Bytes()

		got := &tengo.Bytecode{}
		require.NoError(t, got.Decode(bytes.NewReader(patched), nil))
		require.Equal(t, bc.Constants, got.Constants)
	})

	t.Run("missing DSET is fine", func(t *testing.T) {
		bc := minBytecode(nil)
		var buf bytes.Buffer
		require.NoError(t, bc.Encode(&buf))
		data := buf.Bytes()

		// Strip the DSET section if present.
		dsetOff := bytes.Index(data[8:], []byte("DSET"))
		if dsetOff >= 0 {
			data = data[:dsetOff+8] // truncate at DSET
		}

		got := &tengo.Bytecode{}
		require.NoError(t, got.Decode(bytes.NewReader(data), nil))
		require.NotNil(t, got.FileSet)
	})
}

// ---------------------------------------------------------------------------
// 8. Deterministic encoding
// ---------------------------------------------------------------------------

func TestBytecodeFormat_Deterministic(t *testing.T) {
	t.Run("map keys are always sorted", func(t *testing.T) {
		// Encode the same bytecode twice; the bytes must be identical.
		bc := minBytecode([]tengo.Object{
			&tengo.ImmutableMap{
				Value: map[string]tengo.Object{
					"z": tengo.Int{Value: 26},
					"a": tengo.Int{Value: 1},
					"m": tengo.Int{Value: 13},
					"b": tengo.Int{Value: 2},
				},
			},
		})

		var buf1, buf2 bytes.Buffer
		require.NoError(t, bc.Encode(&buf1))
		require.NoError(t, bc.Encode(&buf2))
		require.Equal(t, buf1.Bytes(), buf2.Bytes())
	})

	t.Run("source map is always sorted by instruction position", func(t *testing.T) {
		fn := &tengo.CompiledFunction{
			Instructions: bytes.Repeat([]byte{byte(parser.OpPop)}, 30),
			SourceMap: map[int]parser.Pos{
				27: 300,
				0:  100,
				9:  200,
				18: 250,
			},
		}
		bc := minBytecode([]tengo.Object{fn})

		var buf1, buf2 bytes.Buffer
		require.NoError(t, bc.Encode(&buf1))
		require.NoError(t, bc.Encode(&buf2))
		require.Equal(t, buf1.Bytes(), buf2.Bytes())
	})
}

// ---------------------------------------------------------------------------
// 9. Kind byte
// ---------------------------------------------------------------------------

func TestBytecodeFormat_KindByte(t *testing.T) {
	bc := minBytecode(nil)

	t.Run("Encode sets script kind", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, bc.Encode(&buf))
		require.Equal(t, int(tengo.BytecodeKindScript), int(tengo.BytecodeDataKind(buf.Bytes())))
	})

	t.Run("EncodeModule sets module kind", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, bc.EncodeModule(&buf))
		require.Equal(t, int(tengo.BytecodeKindModule), int(tengo.BytecodeDataKind(buf.Bytes())))
	})

	t.Run("script-kind file rejected on import", func(t *testing.T) {
		// A file compiled without -module has BytecodeKindScript; the
		// compiler must reject it with a clear message.
		vfs := fstest.MapFS{}

		// Compile a module source and encode it as a SCRIPT (not module).
		modSrc := []byte(`export {"v": 1}`)
		s := tengo.NewScript(modSrc)
		compiled, err := s.Compile()
		require.NoError(t, err)
		var scriptBuf bytes.Buffer
		require.NoError(t, compiled.Bytecode().Encode(&scriptBuf))
		vfs["mod.out"] = &fstest.MapFile{Data: scriptBuf.Bytes()}

		main := tengo.NewScript([]byte(`m := import("mod")`))
		main.SetImportFS(vfs)
		_, err = main.Compile()
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "-module"))
	})

	t.Run("module-kind file accepted on import", func(t *testing.T) {
		vfs := fstest.MapFS{}

		modSrc := []byte(`export {"v": 42}`)
		// Use CompileModuleSrc to get proper module semantics.
		bc, err := tengo.CompileModuleSrc("mod.tengo", modSrc, nil)
		require.NoError(t, err)
		var modBuf bytes.Buffer
		require.NoError(t, bc.EncodeModule(&modBuf))
		vfs["mod.out"] = &fstest.MapFile{Data: modBuf.Bytes()}

		main := tengo.NewScript([]byte(`m := import("mod"); out := m.v`))
		main.SetImportFS(vfs)
		result, err := main.Run()
		require.NoError(t, err)
		require.Equal(t, 42, result.Get("out").Int())
	})
}

// ---------------------------------------------------------------------------
// 10. Large-scale stress
// ---------------------------------------------------------------------------

func TestBytecodeFormat_LargeConstants(t *testing.T) {
	t.Run("1000 integer constants", func(t *testing.T) {
		objs := make([]tengo.Object, 1000)
		for i := range objs {
			objs[i] = tengo.Int{Value: int64(i)}
		}
		bc := minBytecode(objs)
		got := roundTrip(t, bc, nil)
		require.Equal(t, 1000, len(got.Constants))
		for i, c := range got.Constants {
			require.Equal(t, int64(i), c.(tengo.Int).Value)
		}
	})

	t.Run("1 MB string constant", func(t *testing.T) {
		big := strings.Repeat("tengo", 200*1024) // 1 MB
		bc := minBytecode([]tengo.Object{&tengo.String{Value: big}})
		got := roundTrip(t, bc, nil)
		require.Equal(t, big, got.Constants[0].(*tengo.String).Value)
	})

	t.Run("deeply nested array", func(t *testing.T) {
		var obj tengo.Object = tengo.Int{Value: 0}
		for depth := 0; depth < 50; depth++ {
			obj = &tengo.Array{Value: []tengo.Object{obj}}
		}
		bc := minBytecode([]tengo.Object{obj})
		got := roundTrip(t, bc, nil)
		require.Equal(t, bc.Constants, got.Constants)
	})
}

// ---------------------------------------------------------------------------
// 11. Script compile → encode → decode → run integration
// ---------------------------------------------------------------------------

func TestBytecodeFormat_ScriptRoundTrip(t *testing.T) {
	run := func(t *testing.T, src string, modules *tengo.ModuleMap) ([]tengo.Object, *tengo.Bytecode) {
		t.Helper()
		s := tengo.NewScript([]byte(src))
		if modules != nil {
			s.SetImports(modules)
		}
		compiled, err := s.Compile()
		require.NoError(t, err)

		bc := compiled.Bytecode()
		var buf bytes.Buffer
		require.NoError(t, bc.Encode(&buf))

		decoded := &tengo.Bytecode{}
		require.NoError(t, decoded.Decode(bytes.NewReader(buf.Bytes()), modules))

		globals := runBytecode(t, decoded)
		return globals, decoded
	}

	t.Run("arithmetic", func(t *testing.T) {
		globals, _ := run(t, `out := 6 * 7`, nil)
		require.Equal(t, int64(42), globals[0].(tengo.Int).Value)
	})

	t.Run("string concat", func(t *testing.T) {
		globals, _ := run(t, `out := "hello" + ", " + "world"`, nil)
		require.Equal(t, "hello, world", globals[0].(*tengo.String).Value)
	})

	t.Run("float arithmetic", func(t *testing.T) {
		globals, _ := run(t, `out := 1.5 * 2.0`, nil)
		require.Equal(t, 3.0, globals[0].(tengo.Float).Value)
	})

	t.Run("conditional", func(t *testing.T) {
		globals, _ := run(t, `
x := 10
out := 0
if x > 5 { out = 1 } else { out = 2 }
`, nil)
		require.Equal(t, int64(1), globals[1].(tengo.Int).Value)
	})

	t.Run("closure", func(t *testing.T) {
		globals, _ := run(t, `
make_adder := func(n) { return func(x) { return x + n } }
add5 := make_adder(5)
out := add5(37)
`, nil)
		require.Equal(t, int64(42), globals[2].(tengo.Int).Value)
	})

	t.Run("stdlib import", func(t *testing.T) {
		// math.abs operates on floats in the Tengo stdlib.
		mods := stdlib.GetModuleMap("math")
		globals, _ := run(t,
			`math := import("math"); out := math.abs(-3.14)`,
			mods)
		v := globals[1].(tengo.Float).Value
		if v < 3.13 || v > 3.15 {
			t.Errorf("expected math.abs(-3.14) ≈ 3.14, got %v", v)
		}
	})

	t.Run("for loop accumulator", func(t *testing.T) {
		globals, _ := run(t, `
sum := 0
for i := 1; i <= 100; i++ { sum += i }
out := sum
`, nil)
		require.Equal(t, int64(5050), globals[2].(tengo.Int).Value)
	})

	t.Run("array map", func(t *testing.T) {
		globals, _ := run(t, `
arr := [1, 2, 3, 4, 5]
out := 0
for _, v in arr { out += v }
`, nil)
		// arr=globals[0], out=globals[1]
		require.Equal(t, int64(15), globals[1].(tengo.Int).Value)
	})
}

// ---------------------------------------------------------------------------
// 12. Pre-compiled module import end-to-end
// ---------------------------------------------------------------------------

func TestBytecodeFormat_PrecompiledModuleImport(t *testing.T) {
	t.Run("simple export map", func(t *testing.T) {
		modSrc := []byte(`export {"answer": 42, "greeting": "hello"}`)
		bc, err := tengo.CompileModuleSrc("mod.tengo", modSrc, nil)
		require.NoError(t, err)

		var buf bytes.Buffer
		require.NoError(t, bc.EncodeModule(&buf))

		vfs := fstest.MapFS{
			"mod.out": &fstest.MapFile{Data: buf.Bytes()},
		}

		s := tengo.NewScript([]byte(`
m := import("mod")
answer := m.answer
greeting := m.greeting
`))
		s.SetImportFS(vfs)
		result, err := s.Run()
		require.NoError(t, err)
		require.Equal(t, 42, result.Get("answer").Int())
		require.Equal(t, "hello", result.Get("greeting").String())
	})

	t.Run("module that uses stdlib", func(t *testing.T) {
		mods := stdlib.GetModuleMap("math")
		modSrc := []byte(`
math := import("math")
export {"sqrt2": math.sqrt(2.0)}
`)
		bc, err := tengo.CompileModuleSrc("mod.tengo", modSrc, mods)
		require.NoError(t, err)

		var buf bytes.Buffer
		require.NoError(t, bc.EncodeModule(&buf))

		vfs := fstest.MapFS{
			"mod.out": &fstest.MapFile{Data: buf.Bytes()},
		}

		s := tengo.NewScript([]byte(`m := import("mod"); out := m.sqrt2`))
		s.SetImports(mods)
		s.SetImportFS(vfs)
		result, err := s.Run()
		require.NoError(t, err)
		// math.sqrt(2) ≈ 1.4142
		v := result.Get("out").Float()
		if v < 1.41 || v > 1.42 {
			t.Errorf("expected sqrt(2) ≈ 1.414, got %v", v)
		}
	})

	t.Run("module with closure export", func(t *testing.T) {
		modSrc := []byte(`
export func(x) {
    multiplier := 3
    return func(n) { return n * multiplier + x }
}
`)
		bc, err := tengo.CompileModuleSrc("mod.tengo", modSrc, nil)
		require.NoError(t, err)

		var buf bytes.Buffer
		require.NoError(t, bc.EncodeModule(&buf))

		vfs := fstest.MapFS{
			"mod.out": &fstest.MapFile{Data: buf.Bytes()},
		}

		s := tengo.NewScript([]byte(`
make_fn := import("mod")
fn := make_fn(10)
out := fn(4)
`))
		s.SetImportFS(vfs)
		result, err := s.Run()
		require.NoError(t, err)
		// fn(4) = 4*3 + 10 = 22
		require.Equal(t, 22, result.Get("out").Int())
	})

	t.Run("two pre-compiled modules used together", func(t *testing.T) {
		// Compile add and mul independently; mul registers add as a named
		// source module so CompileModuleSrc can resolve the import.
		addSrc := []byte(`export func(a, b) { return a + b }`)
		addBC, err := tengo.CompileModuleSrc("add.tengo", addSrc, nil)
		require.NoError(t, err)
		var addBuf bytes.Buffer
		require.NoError(t, addBC.EncodeModule(&addBuf))

		// mul imports "add" as a named source module during its compilation.
		mulMods := tengo.NewModuleMap()
		mulMods.AddSourceModule("add", addSrc)
		mulSrc := []byte(`
add := import("add")
export func(a, b) {
    result := 0
    for i := 0; i < b; i++ { result = add(result, a) }
    return result
}`)
		mulBC, err := tengo.CompileModuleSrc("mul.tengo", mulSrc, mulMods)
		require.NoError(t, err)
		var mulBuf bytes.Buffer
		require.NoError(t, mulBC.EncodeModule(&mulBuf))

		// At runtime, both modules are resolved from the VFS as pre-compiled
		// .out files. The add module inside mul is inlined at compile time, so
		// the VFS only needs to serve mul.out for the main script.
		vfs := fstest.MapFS{
			"add.out": &fstest.MapFile{Data: addBuf.Bytes()},
			"mul.out": &fstest.MapFile{Data: mulBuf.Bytes()},
		}

		s := tengo.NewScript([]byte(`
add := import("add")
mul := import("mul")
sum := add(6, 7)
product := mul(6, 7)
`))
		s.SetImportFS(vfs)
		result, err := s.Run()
		require.NoError(t, err)
		require.Equal(t, 13, result.Get("sum").Int())
		require.Equal(t, 42, result.Get("product").Int())
	})
}

// ---------------------------------------------------------------------------
// 13. Encode/Decode symmetry with all object types in one bytecode
// ---------------------------------------------------------------------------

func TestBytecodeFormat_FullRoundTrip(t *testing.T) {
	// One bytecode containing every object type we support.
	loc := time.FixedZone("UTC+1", 3600)
	objs := []tengo.Object{
		tengo.UndefinedValue,
		tengo.Int{Value: -9999},
		tengo.Float{Value: math.Pi},
		tengo.TrueValue,
		tengo.FalseValue,
		tengo.Char{Value: '🎉'},
		&tengo.String{Value: "full round-trip"},
		&tengo.Bytes{Value: []byte{1, 2, 3}},
		&tengo.Array{Value: []tengo.Object{
			tengo.Int{Value: 1},
			&tengo.String{Value: "inner"},
		}},
		&tengo.ImmutableArray{Value: []tengo.Object{tengo.TrueValue}},
		&tengo.Map{Value: map[string]tengo.Object{
			"k": tengo.Int{Value: 100},
		}},
		&tengo.ImmutableMap{Value: map[string]tengo.Object{
			"x": tengo.Float{Value: 2.5},
		}},
		&tengo.Error{Value: &tengo.String{Value: "oops"}},
		&tengo.Time{Value: time.Date(2026, 4, 29, 10, 0, 0, 0, loc)},
		&tengo.CompiledFunction{
			Instructions: concatInsts(
				tengo.MakeInstruction(parser.OpConstant, 1),
				tengo.MakeInstruction(parser.OpReturn, 1),
			),
			SourceMap:     map[int]parser.Pos{0: 5},
			NumLocals:     2,
			NumParameters: 1,
			VarArgs:       true,
		},
	}

	bc := minBytecode(objs)
	got := roundTrip(t, bc, nil)
	require.Equal(t, bc.Constants, got.Constants)
}
