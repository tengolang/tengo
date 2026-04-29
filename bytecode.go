package tengo

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"reflect"

	"github.com/tengolang/tengo/v3/parser"
)

// bytecodeMagic is the first four bytes of every encoded Bytecode file.
// The leading ESC byte (0x1B) is never valid at the start of a UTF-8 Tengo
// source file, so its presence unambiguously identifies a compiled binary.
//
// Layout: ESC 'T' 'n' 'g'
var bytecodeMagic = [4]byte{0x1B, 'T', 'n', 'g'}

// bytecodeHeaderLen is the total length of the fixed file header.
const bytecodeHeaderLen = 8

// bytecodeHeaderOffsets records the byte position of each header field.
const (
	hdrOffVersion  = 4 // format version
	hdrOffKind     = 5 // BytecodeKind
	hdrOffReserved = 6 // two reserved bytes (must be zero)
)

// BytecodeFormatVersion is the format version written into every new file.
// Increment this when the serialised layout changes in a backward-incompatible
// way so that Decode can reject stale files with a clear error instead of
// silently producing corrupt state.
const BytecodeFormatVersion byte = 0x01

// BytecodeKind is the kind byte in the file header and records how the file
// was compiled.
type BytecodeKind byte

const (
	// BytecodeKindScript is a standalone script compiled by the normal path.
	// Its MainFunction ends with OpSuspend and cannot be used as a module.
	BytecodeKindScript BytecodeKind = 0x01

	// BytecodeKindModule is a file compiled with module semantics (-module
	// flag). Its MainFunction ends with OpReturn and can be loaded via import().
	BytecodeKindModule BytecodeKind = 0x02
)

// Bytecode is a compiled instructions and constants.
type Bytecode struct {
	FileSet      *parser.SourceFileSet
	MainFunction *CompiledFunction
	Constants    []Object
}

// Size of the bytecode in bytes
// (as much as we can calculate it without reflection and black magic)
func (b *Bytecode) Size() int64 {
	return b.MainFunction.Size() + b.FileSet.Size() + int64(len(b.Constants))
}

// Clone of the bytecode suitable for modification without affecting the original.
// New Bytecode itself is independent, but all the contents of it are still shared
// with the original.
// The only thing that is not shared with the original is Constants slice, as it might be updated
// by ReplaceBuiltinModule(), which should be safe for clone.
func (b *Bytecode) Clone() *Bytecode {
	return &Bytecode{
		FileSet:      b.FileSet,
		MainFunction: b.MainFunction,
		Constants:    append([]Object{}, b.Constants...),
	}
}

// Encode writes Bytecode data to the writer as a script-compiled file.
// The output is prefixed with the four-byte magic header followed by
// BytecodeKindScript so consumers can quickly identify the file type.
//
// To produce a file importable via import(), use EncodeModule instead.
func (b *Bytecode) Encode(w io.Writer) error {
	return b.encode(w, BytecodeKindScript)
}

// EncodeModule writes Bytecode data compiled with module semantics. The kind
// byte is set to BytecodeKindModule, allowing the import resolver to accept
// the file and reject script-compiled files with a clear error.
func (b *Bytecode) EncodeModule(w io.Writer) error {
	return b.encode(w, BytecodeKindModule)
}

func (b *Bytecode) encode(w io.Writer, kind BytecodeKind) error {
	var header [bytecodeHeaderLen]byte
	copy(header[:4], bytecodeMagic[:])
	header[hdrOffVersion] = BytecodeFormatVersion
	header[hdrOffKind] = byte(kind)
	// bytes 6–7 are reserved and remain zero
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	enc := gob.NewEncoder(w)
	if err := enc.Encode(b.FileSet); err != nil {
		return err
	}
	if err := enc.Encode(b.MainFunction); err != nil {
		return err
	}
	return enc.Encode(b.Constants)
}

// IsBytecodeData reports whether data begins with the Tengo bytecode magic
// header. Use this to distinguish compiled files from Tengo source before
// attempting a full Decode.
func IsBytecodeData(data []byte) bool {
	return len(data) >= len(bytecodeMagic) &&
		bytes.Equal(data[:len(bytecodeMagic)], bytecodeMagic[:])
}

// BytecodeDataVersion returns the format version from the header of data
// produced by Encode or EncodeModule. It returns 0 for legacy files that
// pre-date the versioned header.
func BytecodeDataVersion(data []byte) byte {
	if IsBytecodeData(data) && len(data) > hdrOffVersion {
		return data[hdrOffVersion]
	}
	return 0
}

// BytecodeDataKind returns the kind byte from data produced by Encode or
// EncodeModule. It returns BytecodeKindScript for legacy files that pre-date
// the versioned header.
func BytecodeDataKind(data []byte) BytecodeKind {
	if IsBytecodeData(data) && len(data) > hdrOffKind {
		return BytecodeKind(data[hdrOffKind])
	}
	return BytecodeKindScript
}

// CountObjects returns the number of objects found in Constants.
func (b *Bytecode) CountObjects() int {
	n := 0
	for _, c := range b.Constants {
		n += CountObjects(c)
	}
	return n
}

// FormatInstructions returns human readable string representations of
// compiled instructions.
func (b *Bytecode) FormatInstructions() []string {
	return FormatInstructions(b.MainFunction.Instructions, 0)
}

// FormatConstants returns human readable string representations of
// compiled constants.
func (b *Bytecode) FormatConstants() (output []string) {
	for cidx, cn := range b.Constants {
		switch cn := cn.(type) {
		case *CompiledFunction:
			output = append(output, fmt.Sprintf(
				"[% 3d] (Compiled Function|%p)", cidx, &cn))
			for _, l := range FormatInstructions(cn.Instructions, 0) {
				output = append(output, fmt.Sprintf("     %s", l))
			}
		default:
			t := reflect.TypeOf(cn)
			if t.Kind() == reflect.Ptr {
				t = t.Elem()
			}
			output = append(output, fmt.Sprintf("[% 3d] %s (%s|%p)",
				cidx, cn, t.Name(), &cn))
		}
	}
	return
}

// ReplaceBuiltinModule replaces a builtin module with a new one.
// This is helpful for concurrent script execution, when builtin module does not support
// concurrency and you need to provide custom module instance for each script clone.
//
// This method mutates the Bytecode in place and is not safe for concurrent use.
// Prefer Compiled.ReplaceBuiltinModule, which handles copy-on-write and locking.
func (b *Bytecode) ReplaceBuiltinModule(name string, attrs map[string]Object) {
	for i, c := range b.Constants {
		switch c := c.(type) {
		case *ImmutableMap:
			modName := inferModuleName(c)
			if modName == name {
				b.Constants[i] = (&BuiltinModule{Attrs: attrs}).AsImmutableMap(name)
			}
		}
	}
}

// Decode reads Bytecode data from the reader.
// Must only be called before the Bytecode is handed to any VM or Compiled
// instance. Calling Decode on a Bytecode that is already in use is a data race.
//
// Three header generations are handled transparently:
//
//   - Current (8-byte): magic(4) + version(1) + kind(1) + reserved(2)
//   - Previous (5-byte): magic(4) + kind(1)   — no version, no reserved
//   - Legacy  (0-byte): no header at all       — raw gob from before headers
func (b *Bytecode) Decode(r io.Reader, modules *ModuleMap) error {
	if modules == nil {
		modules = NewModuleMap()
	}

	// Read up to bytecodeHeaderLen bytes to inspect the header. ReadFull
	// returns io.ErrUnexpectedEOF when the stream is shorter; that is fine
	// for legacy files and is handled below.
	header := make([]byte, bytecodeHeaderLen)
	n, err := io.ReadFull(r, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		return err
	}
	header = header[:n]

	switch {
	case n >= bytecodeHeaderLen && bytes.Equal(header[:4], bytecodeMagic[:]):
		// Current 8-byte header — all bytes consumed, nothing to push back.

	case n >= 5 && bytes.Equal(header[:4], bytecodeMagic[:]):
		// Previous 5-byte header (magic + kind, no version/reserved).
		// The bytes beyond position 5 are part of the gob stream.
		r = io.MultiReader(bytes.NewReader(header[5:]), r)

	default:
		// Legacy file with no header — push every byte read back.
		r = io.MultiReader(bytes.NewReader(header), r)
	}

	dec := gob.NewDecoder(r)
	if err := dec.Decode(&b.FileSet); err != nil {
		return err
	}
	// TODO: files in b.FileSet.File does not have their 'set' field properly
	//  set to b.FileSet as it's private field and not serialized by gob
	//  encoder/decoder.
	if err := dec.Decode(&b.MainFunction); err != nil {
		return err
	}
	if err := dec.Decode(&b.Constants); err != nil {
		return err
	}
	for i, v := range b.Constants {
		fv, err := fixDecodedObject(v, modules)
		if err != nil {
			return err
		}
		b.Constants[i] = fv
	}
	return nil
}

// RemoveDuplicates finds and remove the duplicate values in Constants.
// Note this function mutates Bytecode, including patching instruction bytes
// in place. Must only be called before the Bytecode is handed to any VM or
// Compiled instance. Calling RemoveDuplicates on a Bytecode that is already
// in use is a data race.
func (b *Bytecode) RemoveDuplicates() {
	var deduped []Object

	indexMap := make(map[int]int) // mapping from old constant index to new index
	fns := make(map[*CompiledFunction]int)
	ints := make(map[int64]int)
	strings := make(map[string]int)
	floats := make(map[float64]int)
	chars := make(map[rune]int)
	immutableMaps := make(map[string]int) // for modules

	for curIdx, c := range b.Constants {
		switch c := c.(type) {
		case *CompiledFunction:
			if newIdx, ok := fns[c]; ok {
				indexMap[curIdx] = newIdx
			} else {
				newIdx = len(deduped)
				fns[c] = newIdx
				indexMap[curIdx] = newIdx
				deduped = append(deduped, c)
			}
		case *ImmutableMap:
			modName := inferModuleName(c)
			newIdx, ok := immutableMaps[modName]
			if modName != "" && ok {
				indexMap[curIdx] = newIdx
			} else {
				newIdx = len(deduped)
				immutableMaps[modName] = newIdx
				indexMap[curIdx] = newIdx
				deduped = append(deduped, c)
			}
		case Int:
			if newIdx, ok := ints[c.Value]; ok {
				indexMap[curIdx] = newIdx
			} else {
				newIdx = len(deduped)
				ints[c.Value] = newIdx
				indexMap[curIdx] = newIdx
				deduped = append(deduped, c)
			}
		case *String:
			if newIdx, ok := strings[c.Value]; ok {
				indexMap[curIdx] = newIdx
			} else {
				newIdx = len(deduped)
				strings[c.Value] = newIdx
				indexMap[curIdx] = newIdx
				deduped = append(deduped, c)
			}
		case Float:
			if newIdx, ok := floats[c.Value]; ok {
				indexMap[curIdx] = newIdx
			} else {
				newIdx = len(deduped)
				floats[c.Value] = newIdx
				indexMap[curIdx] = newIdx
				deduped = append(deduped, c)
			}
		case Char:
			if newIdx, ok := chars[c.Value]; ok {
				indexMap[curIdx] = newIdx
			} else {
				newIdx = len(deduped)
				chars[c.Value] = newIdx
				indexMap[curIdx] = newIdx
				deduped = append(deduped, c)
			}
		default:
			panic(fmt.Errorf("unsupported top-level constant type: %s",
				c.TypeName()))
		}
	}

	// replace with de-duplicated constants
	b.Constants = deduped

	// update CONST instructions with new indexes
	// main function
	updateConstIndexes(b.MainFunction.Instructions, indexMap)
	// other compiled functions in constants
	for _, c := range b.Constants {
		switch c := c.(type) {
		case *CompiledFunction:
			updateConstIndexes(c.Instructions, indexMap)
		}
	}
}

func fixDecodedObject(
	o Object,
	modules *ModuleMap,
) (Object, error) {
	switch o := o.(type) {
	case Bool:
		if o.IsFalsy() {
			return FalseValue, nil
		}
		return TrueValue, nil
	case *Undefined:
		return UndefinedValue, nil
	case *Array:
		for i, v := range o.Value {
			fv, err := fixDecodedObject(v, modules)
			if err != nil {
				return nil, err
			}
			o.Value[i] = fv
		}
	case *ImmutableArray:
		for i, v := range o.Value {
			fv, err := fixDecodedObject(v, modules)
			if err != nil {
				return nil, err
			}
			o.Value[i] = fv
		}
	case *Map:
		for k, v := range o.Value {
			fv, err := fixDecodedObject(v, modules)
			if err != nil {
				return nil, err
			}
			o.Value[k] = fv
		}
	case *ImmutableMap:
		modName := inferModuleName(o)
		if mod := modules.GetBuiltinModule(modName); mod != nil {
			return mod.AsImmutableMap(modName), nil
		}

		for k, v := range o.Value {
			// encoding of user function not supported
			if _, isUserFunction := v.(*UserFunction); isUserFunction {
				return nil, fmt.Errorf("user function not decodable")
			}

			fv, err := fixDecodedObject(v, modules)
			if err != nil {
				return nil, err
			}
			o.Value[k] = fv
		}
	}
	return o, nil
}

func updateConstIndexes(insts []byte, indexMap map[int]int) {
	i := 0
	for i < len(insts) {
		op := insts[i]
		numOperands := parser.OpcodeOperands[op]
		_, read := parser.ReadOperands(numOperands, insts[i+1:])

		switch op {
		case parser.OpConstant:
			curIdx := int(insts[i+2]) | int(insts[i+1])<<8
			newIdx, ok := indexMap[curIdx]
			if !ok {
				panic(fmt.Errorf("constant index not found: %d", curIdx))
			}
			copy(insts[i:], MakeInstruction(op, newIdx))
		case parser.OpClosure:
			curIdx := int(insts[i+2]) | int(insts[i+1])<<8
			numFree := int(insts[i+3])
			newIdx, ok := indexMap[curIdx]
			if !ok {
				panic(fmt.Errorf("constant index not found: %d", curIdx))
			}
			copy(insts[i:], MakeInstruction(op, newIdx, numFree))
		}

		i += 1 + read
	}
}

func inferModuleName(mod *ImmutableMap) string {
	if modName, ok := mod.Value["__module_name__"].(*String); ok {
		return modName.Value
	}
	return ""
}

func init() {
	gob.Register(&parser.SourceFileSet{})
	gob.Register(&parser.SourceFile{})
	gob.Register(&Array{})
	gob.Register(Bool{})
	gob.Register(&Bytes{})
	gob.Register(Char{})
	gob.Register(&CompiledFunction{})
	gob.Register(&Error{})
	gob.Register(Float{})
	gob.Register(&ImmutableArray{})
	gob.Register(&ImmutableMap{})
	gob.Register(Int{})
	gob.Register(&Map{})
	gob.Register(&String{})
	gob.Register(&Time{})
	gob.Register(&Undefined{})
	gob.Register(&UserFunction{})
}
