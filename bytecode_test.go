package tengo_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/ganehag/tengo/v3"
	"github.com/ganehag/tengo/v3/parser"
	"github.com/ganehag/tengo/v3/require"
)

type srcfile struct {
	name string
	size int
}

func TestBytecode(t *testing.T) {
	testBytecodeSerialization(t, bytecode(concatInsts(), objectsArray()))

	testBytecodeSerialization(t, bytecode(
		concatInsts(), objectsArray(
			tengo.Char{Value: 'y'},
			tengo.Float{Value: 93.11},
			compiledFunction(1, 0,
				tengo.MakeInstruction(parser.OpConstant, 3),
				tengo.MakeInstruction(parser.OpSetLocal, 0),
				tengo.MakeInstruction(parser.OpGetGlobal, 0),
				tengo.MakeInstruction(parser.OpGetFree, 0)),
			tengo.Float{Value: 39.2},
			tengo.Int{Value: 192},
			&tengo.String{Value: "bar"})))

	testBytecodeSerialization(t, bytecodeFileSet(
		concatInsts(
			tengo.MakeInstruction(parser.OpConstant, 0),
			tengo.MakeInstruction(parser.OpSetGlobal, 0),
			tengo.MakeInstruction(parser.OpConstant, 6),
			tengo.MakeInstruction(parser.OpPop)),
		objectsArray(
			tengo.Int{Value: 55},
			tengo.Int{Value: 66},
			tengo.Int{Value: 77},
			tengo.Int{Value: 88},
			&tengo.ImmutableMap{
				Value: map[string]tengo.Object{
					"array": &tengo.ImmutableArray{
						Value: []tengo.Object{
							tengo.Int{Value: 1},
							tengo.Int{Value: 2},
							tengo.Int{Value: 3},
							tengo.TrueValue,
							tengo.FalseValue,
							tengo.UndefinedValue,
						},
					},
					"true":  tengo.TrueValue,
					"false": tengo.FalseValue,
					"bytes": &tengo.Bytes{Value: make([]byte, 16)},
					"char":  tengo.Char{Value: 'Y'},
					"error": &tengo.Error{Value: &tengo.String{
						Value: "some error",
					}},
					"float": tengo.Float{Value: -19.84},
					"immutable_array": &tengo.ImmutableArray{
						Value: []tengo.Object{
							tengo.Int{Value: 1},
							tengo.Int{Value: 2},
							tengo.Int{Value: 3},
							tengo.TrueValue,
							tengo.FalseValue,
							tengo.UndefinedValue,
						},
					},
					"immutable_map": &tengo.ImmutableMap{
						Value: map[string]tengo.Object{
							"a": tengo.Int{Value: 1},
							"b": tengo.Int{Value: 2},
							"c": tengo.Int{Value: 3},
							"d": tengo.TrueValue,
							"e": tengo.FalseValue,
							"f": tengo.UndefinedValue,
						},
					},
					"int": tengo.Int{Value: 91},
					"map": &tengo.Map{
						Value: map[string]tengo.Object{
							"a": tengo.Int{Value: 1},
							"b": tengo.Int{Value: 2},
							"c": tengo.Int{Value: 3},
							"d": tengo.TrueValue,
							"e": tengo.FalseValue,
							"f": tengo.UndefinedValue,
						},
					},
					"string":    &tengo.String{Value: "foo bar"},
					"time":      &tengo.Time{Value: time.Now()},
					"undefined": tengo.UndefinedValue,
				},
			},
			compiledFunction(1, 0,
				tengo.MakeInstruction(parser.OpConstant, 3),
				tengo.MakeInstruction(parser.OpSetLocal, 0),
				tengo.MakeInstruction(parser.OpGetGlobal, 0),
				tengo.MakeInstruction(parser.OpGetFree, 0),
				tengo.MakeInstruction(parser.OpBinaryOp, 11),
				tengo.MakeInstruction(parser.OpGetFree, 1),
				tengo.MakeInstruction(parser.OpBinaryOp, 11),
				tengo.MakeInstruction(parser.OpGetLocal, 0),
				tengo.MakeInstruction(parser.OpBinaryOp, 11),
				tengo.MakeInstruction(parser.OpReturn, 1)),
			compiledFunction(1, 0,
				tengo.MakeInstruction(parser.OpConstant, 2),
				tengo.MakeInstruction(parser.OpSetLocal, 0),
				tengo.MakeInstruction(parser.OpGetFree, 0),
				tengo.MakeInstruction(parser.OpGetLocal, 0),
				tengo.MakeInstruction(parser.OpClosure, 4, 2),
				tengo.MakeInstruction(parser.OpReturn, 1)),
			compiledFunction(1, 0,
				tengo.MakeInstruction(parser.OpConstant, 1),
				tengo.MakeInstruction(parser.OpSetLocal, 0),
				tengo.MakeInstruction(parser.OpGetLocal, 0),
				tengo.MakeInstruction(parser.OpClosure, 5, 1),
				tengo.MakeInstruction(parser.OpReturn, 1))),
		fileSet(srcfile{name: "file1", size: 100},
			srcfile{name: "file2", size: 200})))
}

func TestBytecode_RemoveDuplicates(t *testing.T) {
	testBytecodeRemoveDuplicates(t,
		bytecode(
			concatInsts(), objectsArray(
				tengo.Char{Value: 'y'},
				tengo.Float{Value: 93.11},
				compiledFunction(1, 0,
					tengo.MakeInstruction(parser.OpConstant, 3),
					tengo.MakeInstruction(parser.OpSetLocal, 0),
					tengo.MakeInstruction(parser.OpGetGlobal, 0),
					tengo.MakeInstruction(parser.OpGetFree, 0)),
				tengo.Float{Value: 39.2},
				tengo.Int{Value: 192},
				&tengo.String{Value: "bar"})),
		bytecode(
			concatInsts(), objectsArray(
				tengo.Char{Value: 'y'},
				tengo.Float{Value: 93.11},
				compiledFunction(1, 0,
					tengo.MakeInstruction(parser.OpConstant, 3),
					tengo.MakeInstruction(parser.OpSetLocal, 0),
					tengo.MakeInstruction(parser.OpGetGlobal, 0),
					tengo.MakeInstruction(parser.OpGetFree, 0)),
				tengo.Float{Value: 39.2},
				tengo.Int{Value: 192},
				&tengo.String{Value: "bar"})))

	testBytecodeRemoveDuplicates(t,
		bytecode(
			concatInsts(
				tengo.MakeInstruction(parser.OpConstant, 0),
				tengo.MakeInstruction(parser.OpConstant, 1),
				tengo.MakeInstruction(parser.OpConstant, 2),
				tengo.MakeInstruction(parser.OpConstant, 3),
				tengo.MakeInstruction(parser.OpConstant, 4),
				tengo.MakeInstruction(parser.OpConstant, 5),
				tengo.MakeInstruction(parser.OpConstant, 6),
				tengo.MakeInstruction(parser.OpConstant, 7),
				tengo.MakeInstruction(parser.OpConstant, 8),
				tengo.MakeInstruction(parser.OpClosure, 4, 1)),
			objectsArray(
				tengo.Int{Value: 1},
				tengo.Float{Value: 2.0},
				tengo.Char{Value: '3'},
				&tengo.String{Value: "four"},
				compiledFunction(1, 0,
					tengo.MakeInstruction(parser.OpConstant, 3),
					tengo.MakeInstruction(parser.OpConstant, 7),
					tengo.MakeInstruction(parser.OpSetLocal, 0),
					tengo.MakeInstruction(parser.OpGetGlobal, 0),
					tengo.MakeInstruction(parser.OpGetFree, 0)),
				tengo.Int{Value: 1},
				tengo.Float{Value: 2.0},
				tengo.Char{Value: '3'},
				&tengo.String{Value: "four"})),
		bytecode(
			concatInsts(
				tengo.MakeInstruction(parser.OpConstant, 0),
				tengo.MakeInstruction(parser.OpConstant, 1),
				tengo.MakeInstruction(parser.OpConstant, 2),
				tengo.MakeInstruction(parser.OpConstant, 3),
				tengo.MakeInstruction(parser.OpConstant, 4),
				tengo.MakeInstruction(parser.OpConstant, 0),
				tengo.MakeInstruction(parser.OpConstant, 1),
				tengo.MakeInstruction(parser.OpConstant, 2),
				tengo.MakeInstruction(parser.OpConstant, 3),
				tengo.MakeInstruction(parser.OpClosure, 4, 1)),
			objectsArray(
				tengo.Int{Value: 1},
				tengo.Float{Value: 2.0},
				tengo.Char{Value: '3'},
				&tengo.String{Value: "four"},
				compiledFunction(1, 0,
					tengo.MakeInstruction(parser.OpConstant, 3),
					tengo.MakeInstruction(parser.OpConstant, 2),
					tengo.MakeInstruction(parser.OpSetLocal, 0),
					tengo.MakeInstruction(parser.OpGetGlobal, 0),
					tengo.MakeInstruction(parser.OpGetFree, 0)))))

	testBytecodeRemoveDuplicates(t,
		bytecode(
			concatInsts(
				tengo.MakeInstruction(parser.OpConstant, 0),
				tengo.MakeInstruction(parser.OpConstant, 1),
				tengo.MakeInstruction(parser.OpConstant, 2),
				tengo.MakeInstruction(parser.OpConstant, 3),
				tengo.MakeInstruction(parser.OpConstant, 4)),
			objectsArray(
				tengo.Int{Value: 1},
				tengo.Int{Value: 2},
				tengo.Int{Value: 3},
				tengo.Int{Value: 1},
				tengo.Int{Value: 3})),
		bytecode(
			concatInsts(
				tengo.MakeInstruction(parser.OpConstant, 0),
				tengo.MakeInstruction(parser.OpConstant, 1),
				tengo.MakeInstruction(parser.OpConstant, 2),
				tengo.MakeInstruction(parser.OpConstant, 0),
				tengo.MakeInstruction(parser.OpConstant, 2)),
			objectsArray(
				tengo.Int{Value: 1},
				tengo.Int{Value: 2},
				tengo.Int{Value: 3})))

	// Same *CompiledFunction pointer at two different constant indexes (the
	// scenario produced by importing the same source module from two places).
	// Deduplication must collapse the two entries and patch all OpConstant
	// references that pointed to the duplicate.
	sharedFn := compiledFunction(0, 0,
		tengo.MakeInstruction(parser.OpReturn, 0))
	testBytecodeRemoveDuplicates(t,
		bytecode(
			concatInsts(
				tengo.MakeInstruction(parser.OpConstant, 0), // first import
				tengo.MakeInstruction(parser.OpCall, 0, 0),
				tengo.MakeInstruction(parser.OpPop),
				tengo.MakeInstruction(parser.OpConstant, 1), // second import — duplicate
				tengo.MakeInstruction(parser.OpCall, 0, 0),
				tengo.MakeInstruction(parser.OpPop)),
			[]tengo.Object{sharedFn, sharedFn}), // same pointer twice
		bytecode(
			concatInsts(
				tengo.MakeInstruction(parser.OpConstant, 0),
				tengo.MakeInstruction(parser.OpCall, 0, 0),
				tengo.MakeInstruction(parser.OpPop),
				tengo.MakeInstruction(parser.OpConstant, 0), // patched to 0
				tengo.MakeInstruction(parser.OpCall, 0, 0),
				tengo.MakeInstruction(parser.OpPop)),
			[]tengo.Object{sharedFn})) // one entry
}

// TestRemoveDuplicatesSourceModule verifies end-to-end that a source module
// imported from multiple places in the same script produces only a single
// CompiledFunction in the constants after compilation (via Script.Compile,
// which calls RemoveDuplicates internally).
func TestRemoveDuplicatesSourceModule(t *testing.T) {
	modSrc := []byte(`export func(x) { return x * 2 }`)

	// Import the same module from two different variables.
	scriptSrc := []byte(`
double1 := import("double")
double2 := import("double")
a := double1(3)
b := double2(5)
`)
	mods := tengo.NewModuleMap()
	mods.AddSourceModule("double", modSrc)

	s := tengo.NewScript(scriptSrc)
	s.SetImports(mods)

	compiled, err := s.Compile()
	if err != nil {
		t.Fatal(err)
	}

	// Count how many CompiledFunction objects are in the constants. There
	// should be exactly two: the module's outer wrapper function and the
	// inner exported func. Before the fix this would be three (the wrapper
	// was duplicated).
	var fnCount int
	for _, c := range compiled.Bytecode().Constants {
		if _, ok := c.(*tengo.CompiledFunction); ok {
			fnCount++
		}
	}
	if fnCount != 2 {
		t.Errorf("expected 2 CompiledFunction constants after dedup, got %d", fnCount)
	}

	// Correctness: the script must still produce the right results.
	if err := compiled.Run(); err != nil {
		t.Fatal(err)
	}
	if got := compiled.Get("a").Int(); got != 6 {
		t.Errorf("a: want 6, got %d", got)
	}
	if got := compiled.Get("b").Int(); got != 10 {
		t.Errorf("b: want 10, got %d", got)
	}
}

func TestBytecode_CountObjects(t *testing.T) {
	b := bytecode(
		concatInsts(),
		objectsArray(
			tengo.Int{Value: 55},
			tengo.Int{Value: 66},
			tengo.Int{Value: 77},
			tengo.Int{Value: 88},
			compiledFunction(1, 0,
				tengo.MakeInstruction(parser.OpConstant, 3),
				tengo.MakeInstruction(parser.OpReturn, 1)),
			compiledFunction(1, 0,
				tengo.MakeInstruction(parser.OpConstant, 2),
				tengo.MakeInstruction(parser.OpReturn, 1)),
			compiledFunction(1, 0,
				tengo.MakeInstruction(parser.OpConstant, 1),
				tengo.MakeInstruction(parser.OpReturn, 1))))
	require.Equal(t, 7, b.CountObjects())
}

func fileSet(files ...srcfile) *parser.SourceFileSet {
	fileSet := parser.NewFileSet()
	for _, f := range files {
		fileSet.AddFile(f.name, -1, f.size)
	}
	return fileSet
}

func bytecodeFileSet(
	instructions []byte,
	constants []tengo.Object,
	fileSet *parser.SourceFileSet,
) *tengo.Bytecode {
	return &tengo.Bytecode{
		FileSet:      fileSet,
		MainFunction: &tengo.CompiledFunction{Instructions: instructions},
		Constants:    constants,
	}
}

func testBytecodeRemoveDuplicates(
	t *testing.T,
	input, expected *tengo.Bytecode,
) {
	input.RemoveDuplicates()

	require.Equal(t, expected.FileSet, input.FileSet)
	require.Equal(t, expected.MainFunction, input.MainFunction)
	require.Equal(t, expected.Constants, input.Constants)
}

func testBytecodeSerialization(t *testing.T, b *tengo.Bytecode) {
	var buf bytes.Buffer
	err := b.Encode(&buf)
	require.NoError(t, err)

	r := &tengo.Bytecode{}
	err = r.Decode(bytes.NewReader(buf.Bytes()), nil)
	require.NoError(t, err)

	require.Equal(t, b.FileSet, r.FileSet)
	require.Equal(t, b.MainFunction, r.MainFunction)
	require.Equal(t, b.Constants, r.Constants)
}
