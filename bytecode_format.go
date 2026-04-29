package tengo

// bytecode_format.go — section-based binary encoding for compiled Bytecode.
//
// File layout (after the 8-byte header written by Bytecode.encode):
//
//	[CONS section]  — constants pool            (mandatory)
//	[MAIN section]  — entry-point function       (mandatory)
//	[DSET section]  — source file set for traces (optional)
//
// Each section:
//
//	[4-byte ASCII tag]
//	[4-byte uint32 data length, big-endian]
//	[data bytes]
//
// Unknown section tags are silently skipped, enabling forward compatibility.
// All multi-byte integers are big-endian throughout.
//
// Object type tags (one byte):
//
//	0x00  Undefined
//	0x01  Int            int64  (8 bytes BE)
//	0x02  Float          float64 IEEE 754 (8 bytes BE)
//	0x03  Bool           uint8 (0=false, 1=true)
//	0x04  Char           int32  (4 bytes BE)
//	0x05  String         uint32 length + UTF-8 bytes
//	0x06  Bytes          uint32 length + raw bytes
//	0x07  Array          uint32 count + count × Object
//	0x08  ImmutableArray uint32 count + count × Object
//	0x09  Map            uint32 count + count × (String + Object)
//	0x0A  ImmutableMap   uint32 count + count × (String + Object)
//	0x0B  CompiledFunction — see encodeCompiledFunction
//	0x0C  ModuleRef      uint32 name-length + name bytes
//	             (builtin module stored by name; resolved at decode time)
//
// CompiledFunction layout (inside MAIN or tagged 0x0B in CONS):
//
//	uint32   instruction_length
//	bytes    instructions
//	uint32   source_map_entry_count
//	  per entry (sorted ascending by instr_pos):
//	    uint32  instr_pos
//	    int32   source_pos  (parser.Pos)
//	uint32   num_locals
//	uint32   num_parameters
//	uint8    varargs  (0=false, 1=true)
//
// DSET section (source file set):
//
//	uint32   file_count
//	  per file:
//	    uint16  name_length
//	    bytes   name (UTF-8)
//	    int32   base
//	    int32   size
//	    uint32  line_count
//	      per line: int32 offset

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sort"
	"time"

	"github.com/tengolang/tengo/v3/parser"
)

// section tags
var (
	secCONS = [4]byte{'C', 'O', 'N', 'S'} // constants pool
	secMAIN = [4]byte{'M', 'A', 'I', 'N'} // main compiled function
	secDSET = [4]byte{'D', 'S', 'E', 'T'} // debug: source file set
)

// object type tags
const (
	tagUndefined        byte = 0x00
	tagInt              byte = 0x01
	tagFloat            byte = 0x02
	tagBool             byte = 0x03
	tagChar             byte = 0x04
	tagString           byte = 0x05
	tagBytes            byte = 0x06
	tagArray            byte = 0x07
	tagImmutableArray   byte = 0x08
	tagMap              byte = 0x09
	tagImmutableMap     byte = 0x0A
	tagCompiledFunction byte = 0x0B
	tagModuleRef        byte = 0x0C
	tagError            byte = 0x0D
	tagTime             byte = 0x0E
)

// ---------------------------------------------------------------------------
// Top-level encode / decode
// ---------------------------------------------------------------------------

// encodeV2 writes the version-2 section-based payload to w. The 8-byte header
// has already been written by encode().
func (b *Bytecode) encodeV2(w io.Writer) error {
	if err := writeSection(w, secCONS, func(buf io.Writer) error {
		return encodeConstants(buf, b.Constants)
	}); err != nil {
		return err
	}

	if err := writeSection(w, secMAIN, func(buf io.Writer) error {
		return encodeCompiledFunction(buf, b.MainFunction)
	}); err != nil {
		return err
	}

	if b.FileSet != nil && len(b.FileSet.Files) > 0 {
		if err := writeSection(w, secDSET, func(buf io.Writer) error {
			return encodeFileSet(buf, b.FileSet)
		}); err != nil {
			return err
		}
	}

	return nil
}

// decodeV2 reads the version-2 section-based payload from r. The 8-byte
// header has already been consumed by Decode().
func (b *Bytecode) decodeV2(r io.Reader, modules *ModuleMap) error {
	for {
		var tag [4]byte
		if _, err := io.ReadFull(r, tag[:]); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return fmt.Errorf("reading section tag: %w", err)
		}

		var length uint32
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return fmt.Errorf("reading section length: %w", err)
		}

		// Isolate the section into its own reader so parsers cannot
		// overrun into the next section.
		data := make([]byte, length)
		if _, err := io.ReadFull(r, data); err != nil {
			return fmt.Errorf("reading section data for %s: %w", tag, err)
		}
		sr := bytes.NewReader(data)

		switch tag {
		case secCONS:
			constants, err := decodeConstants(sr, modules)
			if err != nil {
				return fmt.Errorf("decoding CONS: %w", err)
			}
			b.Constants = constants

		case secMAIN:
			fn, err := decodeCompiledFunction(sr)
			if err != nil {
				return fmt.Errorf("decoding MAIN: %w", err)
			}
			b.MainFunction = fn

		case secDSET:
			fset, err := decodeFileSet(sr)
			if err != nil {
				return fmt.Errorf("decoding DSET: %w", err)
			}
			b.FileSet = fset

		default:
			// Unknown section — already read into data; skip silently.
		}
	}

	if b.MainFunction == nil {
		return fmt.Errorf("bytecode missing required MAIN section")
	}
	if b.FileSet == nil {
		b.FileSet = parser.NewFileSet()
	}
	return nil
}

// ---------------------------------------------------------------------------
// Sections
// ---------------------------------------------------------------------------

// writeSection writes a section tag, its data length, then calls fn to write
// the data. fn writes into a buffer so the length is known before the header.
func writeSection(w io.Writer, tag [4]byte, fn func(io.Writer) error) error {
	var buf bytes.Buffer
	if err := fn(&buf); err != nil {
		return err
	}
	if _, err := w.Write(tag[:]); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint32(buf.Len())); err != nil {
		return err
	}
	_, err := buf.WriteTo(w)
	return err
}

// ---------------------------------------------------------------------------
// Constants pool
// ---------------------------------------------------------------------------

func encodeConstants(w io.Writer, constants []Object) error {
	if err := writeUint32(w, uint32(len(constants))); err != nil {
		return err
	}
	for i, obj := range constants {
		if err := encodeObject(w, obj); err != nil {
			return fmt.Errorf("constant %d (%T): %w", i, obj, err)
		}
	}
	return nil
}

func decodeConstants(r io.Reader, modules *ModuleMap) ([]Object, error) {
	count, err := readUint32(r)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		// Preserve nil so round-trip equality holds for empty constant pools.
		return nil, nil
	}
	constants := make([]Object, count)
	for i := range constants {
		obj, err := decodeObject(r, modules)
		if err != nil {
			return nil, fmt.Errorf("constant %d: %w", i, err)
		}
		constants[i] = obj
	}
	return constants, nil
}

// ---------------------------------------------------------------------------
// Objects
// ---------------------------------------------------------------------------

func encodeObject(w io.Writer, obj Object) error {
	switch v := obj.(type) {
	case *Undefined:
		return writeByte(w, tagUndefined)

	case Int:
		if err := writeByte(w, tagInt); err != nil {
			return err
		}
		return writeInt64(w, v.Value)

	case Float:
		if err := writeByte(w, tagFloat); err != nil {
			return err
		}
		return writeFloat64(w, v.Value)

	case Bool:
		if err := writeByte(w, tagBool); err != nil {
			return err
		}
		b := byte(0)
		if !v.IsFalsy() {
			b = 1
		}
		return writeByte(w, b)

	case Char:
		if err := writeByte(w, tagChar); err != nil {
			return err
		}
		return writeInt32(w, int32(v.Value))

	case *String:
		if err := writeByte(w, tagString); err != nil {
			return err
		}
		return writeString(w, v.Value)

	case *Bytes:
		if err := writeByte(w, tagBytes); err != nil {
			return err
		}
		return writeByteSlice(w, v.Value)

	case *Array:
		if err := writeByte(w, tagArray); err != nil {
			return err
		}
		return encodeObjectSlice(w, v.Value)

	case *ImmutableArray:
		if err := writeByte(w, tagImmutableArray); err != nil {
			return err
		}
		return encodeObjectSlice(w, v.Value)

	case *Map:
		if err := writeByte(w, tagMap); err != nil {
			return err
		}
		return encodeObjectMap(w, v.Value)

	case *ImmutableMap:
		// Builtin modules (stdlib) contain UserFunction values which cannot
		// be serialised. Store only the module name; it is resolved back to
		// the live module at decode time.
		if name := inferModuleName(v); name != "" {
			if err := writeByte(w, tagModuleRef); err != nil {
				return err
			}
			return writeString(w, name)
		}
		if err := writeByte(w, tagImmutableMap); err != nil {
			return err
		}
		return encodeObjectMap(w, v.Value)

	case *CompiledFunction:
		if err := writeByte(w, tagCompiledFunction); err != nil {
			return err
		}
		return encodeCompiledFunction(w, v)

	case *Error:
		if err := writeByte(w, tagError); err != nil {
			return err
		}
		return encodeObject(w, v.Value)

	case *Time:
		if err := writeByte(w, tagTime); err != nil {
			return err
		}
		// MarshalBinary strips the monotonic clock reading so round-trip
		// equality holds, matching the behaviour of encoding/gob.
		b, err := v.Value.MarshalBinary()
		if err != nil {
			return err
		}
		return writeByteSlice(w, b)

	default:
		return fmt.Errorf("unsupported constant type: %T", obj)
	}
}

func decodeObject(r io.Reader, modules *ModuleMap) (Object, error) {
	tag, err := readByte(r)
	if err != nil {
		return nil, err
	}

	switch tag {
	case tagUndefined:
		return UndefinedValue, nil

	case tagInt:
		v, err := readInt64(r)
		return Int{Value: v}, err

	case tagFloat:
		v, err := readFloat64(r)
		return Float{Value: v}, err

	case tagBool:
		b, err := readByte(r)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			return FalseValue, nil
		}
		return TrueValue, nil

	case tagChar:
		v, err := readInt32(r)
		return Char{Value: rune(v)}, err

	case tagString:
		s, err := readString(r)
		return &String{Value: s}, err

	case tagBytes:
		b, err := readByteSlice(r)
		return &Bytes{Value: b}, err

	case tagArray:
		elems, err := decodeObjectSlice(r, modules)
		return &Array{Value: elems}, err

	case tagImmutableArray:
		elems, err := decodeObjectSlice(r, modules)
		return &ImmutableArray{Value: elems}, err

	case tagMap:
		m, err := decodeObjectMap(r, modules)
		return &Map{Value: m}, err

	case tagImmutableMap:
		m, err := decodeObjectMap(r, modules)
		return &ImmutableMap{Value: m}, err

	case tagCompiledFunction:
		return decodeCompiledFunction(r)

	case tagModuleRef:
		name, err := readString(r)
		if err != nil {
			return nil, err
		}
		if modules != nil {
			if mod := modules.GetBuiltinModule(name); mod != nil {
				return mod.AsImmutableMap(name), nil
			}
		}
		return nil, fmt.Errorf("module %q not found in module map; "+
			"ensure the required stdlib modules are passed to Decode", name)

	case tagError:
		val, err := decodeObject(r, modules)
		if err != nil {
			return nil, err
		}
		return &Error{Value: val}, nil

	case tagTime:
		b, err := readByteSlice(r)
		if err != nil {
			return nil, err
		}
		var t time.Time
		if err := t.UnmarshalBinary(b); err != nil {
			return nil, fmt.Errorf("decoding time: %w", err)
		}
		return &Time{Value: t}, nil

	default:
		return nil, fmt.Errorf("unknown object type tag 0x%02x", tag)
	}
}

func encodeObjectSlice(w io.Writer, objs []Object) error {
	if err := writeUint32(w, uint32(len(objs))); err != nil {
		return err
	}
	for _, obj := range objs {
		if err := encodeObject(w, obj); err != nil {
			return err
		}
	}
	return nil
}

func decodeObjectSlice(r io.Reader, modules *ModuleMap) ([]Object, error) {
	count, err := readUint32(r)
	if err != nil {
		return nil, err
	}
	objs := make([]Object, count)
	for i := range objs {
		obj, err := decodeObject(r, modules)
		if err != nil {
			return nil, err
		}
		objs[i] = obj
	}
	return objs, nil
}

// encodeObjectMap encodes a map with keys sorted for deterministic output.
func encodeObjectMap(w io.Writer, m map[string]Object) error {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if err := writeUint32(w, uint32(len(keys))); err != nil {
		return err
	}
	for _, k := range keys {
		if err := writeString(w, k); err != nil {
			return err
		}
		if err := encodeObject(w, m[k]); err != nil {
			return err
		}
	}
	return nil
}

func decodeObjectMap(r io.Reader, modules *ModuleMap) (map[string]Object, error) {
	count, err := readUint32(r)
	if err != nil {
		return nil, err
	}
	m := make(map[string]Object, count)
	for i := uint32(0); i < count; i++ {
		k, err := readString(r)
		if err != nil {
			return nil, err
		}
		v, err := decodeObject(r, modules)
		if err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// CompiledFunction
// ---------------------------------------------------------------------------

func encodeCompiledFunction(w io.Writer, fn *CompiledFunction) error {
	// instructions
	if err := writeByteSlice(w, fn.Instructions); err != nil {
		return err
	}

	// source map — sorted by instruction position for deterministic output
	keys := make([]int, 0, len(fn.SourceMap))
	for k := range fn.SourceMap {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	if err := writeUint32(w, uint32(len(keys))); err != nil {
		return err
	}
	for _, k := range keys {
		if err := writeUint32(w, uint32(k)); err != nil {
			return err
		}
		if err := writeInt32(w, int32(fn.SourceMap[k])); err != nil {
			return err
		}
	}

	// metadata
	if err := writeUint32(w, uint32(fn.NumLocals)); err != nil {
		return err
	}
	if err := writeUint32(w, uint32(fn.NumParameters)); err != nil {
		return err
	}
	varargs := byte(0)
	if fn.VarArgs {
		varargs = 1
	}
	return writeByte(w, varargs)
}

func decodeCompiledFunction(r io.Reader) (*CompiledFunction, error) {
	insts, err := readByteSlice(r)
	if err != nil {
		return nil, fmt.Errorf("instructions: %w", err)
	}

	smCount, err := readUint32(r)
	if err != nil {
		return nil, fmt.Errorf("source map count: %w", err)
	}

	var sourceMap map[int]parser.Pos
	if smCount > 0 {
		sourceMap = make(map[int]parser.Pos, smCount)
		for i := uint32(0); i < smCount; i++ {
			instrPos, err := readUint32(r)
			if err != nil {
				return nil, err
			}
			srcPos, err := readInt32(r)
			if err != nil {
				return nil, err
			}
			sourceMap[int(instrPos)] = parser.Pos(srcPos)
		}
	}

	numLocals, err := readUint32(r)
	if err != nil {
		return nil, err
	}
	numParams, err := readUint32(r)
	if err != nil {
		return nil, err
	}
	varargsByte, err := readByte(r)
	if err != nil {
		return nil, err
	}

	return &CompiledFunction{
		Instructions:  insts,
		SourceMap:     sourceMap,
		NumLocals:     int(numLocals),
		NumParameters: int(numParams),
		VarArgs:       varargsByte != 0,
	}, nil
}

// ---------------------------------------------------------------------------
// Source file set (DSET section)
// ---------------------------------------------------------------------------

func encodeFileSet(w io.Writer, fset *parser.SourceFileSet) error {
	if err := writeUint32(w, uint32(len(fset.Files))); err != nil {
		return err
	}
	for _, f := range fset.Files {
		if err := writeString16(w, f.Name); err != nil {
			return err
		}
		if err := writeInt32(w, int32(f.Base)); err != nil {
			return err
		}
		if err := writeInt32(w, int32(f.Size)); err != nil {
			return err
		}
		if err := writeUint32(w, uint32(len(f.Lines))); err != nil {
			return err
		}
		for _, line := range f.Lines {
			if err := writeInt32(w, int32(line)); err != nil {
				return err
			}
		}
	}
	return nil
}

func decodeFileSet(r io.Reader) (*parser.SourceFileSet, error) {
	fset := parser.NewFileSet()

	fileCount, err := readUint32(r)
	if err != nil {
		return nil, err
	}

	for i := uint32(0); i < fileCount; i++ {
		name, err := readString16(r)
		if err != nil {
			return nil, err
		}
		base, err := readInt32(r)
		if err != nil {
			return nil, err
		}
		size, err := readInt32(r)
		if err != nil {
			return nil, err
		}
		lineCount, err := readUint32(r)
		if err != nil {
			return nil, err
		}
		lines := make([]int, lineCount)
		for j := range lines {
			v, err := readInt32(r)
			if err != nil {
				return nil, err
			}
			lines[j] = int(v)
		}

		// AddFile sets the private set pointer on the returned SourceFile.
		// We then overwrite Lines to restore the original line table.
		f := fset.AddFile(name, int(base), int(size))
		f.Lines = lines
	}

	return fset, nil
}

// ---------------------------------------------------------------------------
// Primitive write helpers
// ---------------------------------------------------------------------------

func writeByte(w io.Writer, b byte) error {
	_, err := w.Write([]byte{b})
	return err
}

func writeUint32(w io.Writer, v uint32) error {
	return binary.Write(w, binary.BigEndian, v)
}

func writeInt32(w io.Writer, v int32) error {
	return binary.Write(w, binary.BigEndian, v)
}

func writeInt64(w io.Writer, v int64) error {
	return binary.Write(w, binary.BigEndian, v)
}

func writeFloat64(w io.Writer, v float64) error {
	return binary.Write(w, binary.BigEndian, math.Float64bits(v))
}

func writeString(w io.Writer, s string) error {
	if err := writeUint32(w, uint32(len(s))); err != nil {
		return err
	}
	_, err := io.WriteString(w, s)
	return err
}

// writeString16 writes a string with a uint16 length prefix.
// Used for file names in the DSET section where 65535 bytes is always enough.
func writeString16(w io.Writer, s string) error {
	if len(s) > 0xFFFF {
		return fmt.Errorf("file name too long (%d bytes, max 65535)", len(s))
	}
	if err := binary.Write(w, binary.BigEndian, uint16(len(s))); err != nil {
		return err
	}
	_, err := io.WriteString(w, s)
	return err
}

func writeByteSlice(w io.Writer, b []byte) error {
	if err := writeUint32(w, uint32(len(b))); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}

// ---------------------------------------------------------------------------
// Primitive read helpers
// ---------------------------------------------------------------------------

func readByte(r io.Reader) (byte, error) {
	var b [1]byte
	_, err := io.ReadFull(r, b[:])
	return b[0], err
}

func readUint32(r io.Reader) (uint32, error) {
	var v uint32
	err := binary.Read(r, binary.BigEndian, &v)
	return v, err
}

func readInt32(r io.Reader) (int32, error) {
	var v int32
	err := binary.Read(r, binary.BigEndian, &v)
	return v, err
}

func readInt64(r io.Reader) (int64, error) {
	var v int64
	err := binary.Read(r, binary.BigEndian, &v)
	return v, err
}

func readFloat64(r io.Reader) (float64, error) {
	var bits uint64
	if err := binary.Read(r, binary.BigEndian, &bits); err != nil {
		return 0, err
	}
	return math.Float64frombits(bits), nil
}

func readString(r io.Reader) (string, error) {
	length, err := readUint32(r)
	if err != nil {
		return "", err
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func readString16(r io.Reader) (string, error) {
	var length uint16
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return "", err
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func readByteSlice(r io.Reader) ([]byte, error) {
	length, err := readUint32(r)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}
