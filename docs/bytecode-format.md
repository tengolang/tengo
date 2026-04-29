# Tengo Bytecode Format (version 0x02)

This document is the authoritative specification for the binary file format
produced by `Bytecode.Encode` and `Bytecode.EncodeModule`. The format is
intentionally simple so that a VM in any language can consume it without a Go
dependency.

---

## Motivation

The previous format used Go's `encoding/gob`, which is Go-specific, struct-sensitive
(any field rename silently breaks existing files), and unable to encode Go function
values (meaning stdlib module references were effectively unserialised).
The current format replaces gob with a self-describing, big-endian binary layout
that is portable, versioned, and fully specified here.

---

## File layout

```
┌─────────────────────────────────────────────────────┐
│  8-byte file header                                  │
├─────────────────────────────────────────────────────┤
│  CONS section  (mandatory)                           │
├─────────────────────────────────────────────────────┤
│  MAIN section  (mandatory)                           │
├─────────────────────────────────────────────────────┤
│  DSET section  (optional)                            │
├─────────────────────────────────────────────────────┤
│  … additional sections (unknown tags skipped) …     │
└─────────────────────────────────────────────────────┘
```

All multi-byte integers are **big-endian** (network byte order).

---

## File header (8 bytes)

| Offset | Size | Field            | Value / Notes                                    |
|--------|------|------------------|--------------------------------------------------|
| 0      | 4    | Magic            | `1B 54 6E 67` — ESC `T` `n` `g`                 |
| 4      | 1    | Format version   | `0x02` (current). Increment on breaking changes. |
| 5      | 1    | Kind             | `0x01` = script, `0x02` = module (see below)     |
| 6      | 2    | Reserved         | Must be `00 00`. Reserved for future flags.      |

The leading `ESC` byte (`0x1B`) can never appear at the start of valid UTF-8
Tengo source, so `IsBytecodeData(data)` can identify compiled files instantly
without attempting a full decode.

### Kind byte

| Value  | Constant              | Meaning                                                        |
|--------|-----------------------|----------------------------------------------------------------|
| `0x01` | `BytecodeKindScript`  | Compiled as a standalone script. `import()` will reject it.   |
| `0x02` | `BytecodeKindModule`  | Compiled with `-module` flag. `import()` will accept it.      |

Use `tengo -module -o mod.out mod.tengo` to produce a module-kind file.

### Version history

| Version | Description                                    |
|---------|------------------------------------------------|
| `0x01`  | gob-encoded payload (no longer supported)      |
| `0x02`  | Section-based custom binary format (current)   |

---

## Section format

Every section (including unknown future sections) follows the same envelope:

```
┌───────────────────────────────────────────────────┐
│  tag     4 bytes  ASCII identifier e.g. "CONS"    │
│  length  4 bytes  uint32 — byte count of data     │
│  data    N bytes  section-specific content        │
└───────────────────────────────────────────────────┘
```

A decoder that encounters an unknown tag **must** skip it by reading `length`
bytes and continuing. This is the forward-compatibility mechanism — new optional
sections can be added in a future `0x02` file without breaking older decoders.

---

## CONS — constants pool

The constants pool holds every compile-time value referenced by `OpConstant`
instructions in the bytecode.

```
uint32   count          — number of objects that follow
Object×count            — each object encoded as described below
```

### Object encoding

Each object begins with a 1-byte type tag followed by type-specific data.

| Tag    | Type            | Payload                                                         |
|--------|-----------------|------------------------------------------------------------------|
| `0x00` | Undefined       | *(no data)*                                                      |
| `0x01` | Int             | `int64` — 8 bytes, big-endian                                    |
| `0x02` | Float           | `float64` — 8 bytes, IEEE 754 big-endian                         |
| `0x03` | Bool            | `uint8` — `0x00` = false, `0x01` = true                         |
| `0x04` | Char            | `int32` — 4 bytes, big-endian (Unicode code point)              |
| `0x05` | String          | `uint32` length + UTF-8 bytes                                    |
| `0x06` | Bytes           | `uint32` length + raw bytes                                      |
| `0x07` | Array           | `uint32` count + *count* × Object                               |
| `0x08` | ImmutableArray  | `uint32` count + *count* × Object                               |
| `0x09` | Map             | `uint32` count + *count* × (String key + Object value)          |
| `0x0A` | ImmutableMap    | `uint32` count + *count* × (String key + Object value)          |
| `0x0B` | CompiledFunction| see [CompiledFunction encoding](#compiledfunction-encoding)      |
| `0x0C` | ModuleRef       | `uint32` name-length + name bytes (resolved at decode time)      |
| `0x0D` | Error           | 1 × Object (the wrapped error value)                            |
| `0x0E` | Time            | `uint32` length + `time.MarshalBinary()` bytes (15 bytes)       |

**Notes:**

- `Bool`, `Undefined`: decoders should return the canonical singleton values
  rather than allocating new instances, so pointer equality checks work.
- `Map` / `ImmutableMap`: keys are always **sorted lexicographically** when
  encoding, guaranteeing deterministic output across runs.
- `ModuleRef` stores only the module name. At decode time the decoder looks up
  the name in the supplied module map and substitutes the live module object.
  This avoids serialising Go function values (`UserFunction`), which cannot be
  encoded. If the module is not found in the module map, decode returns an error.
- `Time` uses Go's `time.Time.MarshalBinary()` / `UnmarshalBinary()` encoding,
  which is 15 bytes and strips the monotonic clock reading so round-trip
  equality holds.

### String encoding (used inside Map/ImmutableMap keys)

Map keys are encoded as plain strings: `uint32` length + UTF-8 bytes — identical
to tag `0x05` but without the leading type tag byte, since the type is implicit.

---

## MAIN — entry-point compiled function

The MAIN section contains exactly one `CompiledFunction` encoding (without the
`0x0B` type tag — the tag is implicit from the section header).

---

## CompiledFunction encoding

Used both as the MAIN section body and as the payload for tag `0x0B` in the
constants pool (nested functions / lambdas).

```
uint32   instruction_length      — byte count of the instruction stream
bytes    instructions            — raw VM instruction bytes
uint32   source_map_entry_count
  per entry (sorted ascending by instr_pos):
    uint32   instr_pos           — byte offset in instruction stream
    int32    source_pos          — parser.Pos (abstract source position)
uint32   num_locals              — total local variable slots
uint32   num_parameters          — number of declared parameters
uint8    varargs                 — 0 = fixed-arity, 1 = variadic
```

Source map entries are sorted by `instr_pos` for deterministic output.
The `Free` field (`[]*ObjectPtr` for closure variables) is always empty at
compile time and is not serialised; it is populated at runtime when the VM
executes `OpClosure`.

---

## DSET — source file set (optional)

The DSET section records the source file names and line-number tables needed to
produce `file:line` information in runtime error messages. It may be omitted
entirely (e.g. in a stripped release build). A decoder that finds no DSET
section should substitute an empty `SourceFileSet`.

```
uint32   file_count
  per file:
    uint16   name_length         — byte count of the file name
    bytes    name                — UTF-8 file name
    int32    base                — base position in the global position space
    int32    size                — source file size in bytes
    uint32   line_count
      per line: int32 offset     — byte offset of line start (sorted ascending)
```

Files are encoded in the order they were added to the `SourceFileSet` (i.e.
ascending `base`), which is required for correct reconstruction since each
`AddFile` call advances the global base counter.

---

## Primitive types reference

All integers are unsigned unless stated otherwise.

| Name    | Size | Encoding              |
|---------|------|-----------------------|
| uint8   | 1    | big-endian            |
| uint16  | 2    | big-endian            |
| uint32  | 4    | big-endian            |
| int32   | 4    | big-endian two's comp |
| int64   | 8    | big-endian two's comp |
| float64 | 8    | IEEE 754 big-endian   |

---

## Producing module files

To create a bytecode file that can be loaded via `import()`:

```bash
# Compile mod.tengo as a module (export statement is honoured)
tengo -module -o mod.out mod.tengo

# The resulting mod.out can now be imported by other scripts
# without the source file being present.
```

From Go:

```go
bc, err := tengo.CompileModuleSrc("mod.tengo", src, modules)
if err != nil { ... }

f, _ := os.Create("mod.out")
err = bc.EncodeModule(f)
```

---

## Implementing a decoder in another language

The minimum viable decoder needs to:

1. Read and validate the 8-byte header (check magic, version, kind).
2. Loop over sections until EOF: read 4-byte tag + 4-byte length, read `length`
   bytes, parse known tags, skip unknown ones.
3. For CONS: read `uint32` count, then decode each Object by its type tag.
4. For MAIN: decode one CompiledFunction (without a type tag prefix).
5. For DSET (optional): decode the source file set for error reporting.

The only non-obvious part is `ModuleRef` (tag `0x0C`): the decoder must
maintain a map of module names to their native representations and substitute
them at decode time, since Go function values cannot be serialised.
