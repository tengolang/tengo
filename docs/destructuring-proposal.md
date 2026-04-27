# Proposal: Map and import destructuring

## Motivation

Tengo already supports multi-value destructuring of function returns:

```tengo
a, b, c := fn()
```

But there is no equivalent for unpacking a map or module into named
variables. The workaround today is verbose:

```tengo
math := import("math")
pow10 := math.pow10
max   := math.max
min   := math.min
```

The same verbosity appears whenever a function returns a map of results.
Destructuring syntax would eliminate this boilerplate, mirror the way
`import` is commonly used, and align with patterns familiar from JavaScript
and Python.

## Proposed syntax

### Map destructuring (define)

```tengo
{pow10, max, min} := import("math")
```

Binds new local/global variables `pow10`, `max`, and `min` to the
corresponding keys in the map returned by the RHS.

### Map destructuring (assign)

```tengo
{pow10, max, min} = import("math")
```

Assigns to already-declared variables.

### Rename (alias)

```tengo
{max: localMax, min: localMin} := import("math")
```

`key: name` syntax extracts key `max` but binds it to `localMax`.

### Default values

```tengo
{timeout: t = 30, retries: r = 3} := config
```

If the key is absent (or `undefined`), the variable receives the default.

### Nested destructuring

```tengo
{limits: {max_retries, timeout}} := config
```

Recurses into a nested map.

## Semantics

- The RHS must evaluate to a `Map` or `ImmutableMap`; any other type is a
  runtime error.
- Keys not present in the map produce `undefined` (or the declared default).
- Extra keys in the map that are not listed on the LHS are silently ignored.
- `_` as a name discards the key (useful with rename: `{verbose: _} := opts`
  to assert the key exists without binding it).
- Mutability of the result follows normal `:=` / `=` rules; the map itself
  is not modified.

## Examples

```tengo
// Unpack a stdlib module
{abs, pow, sqrt} := import("math")
x := sqrt(abs(-16.0))

// Unpack a function result
parse := func(s) {
    return {value: int(s), ok: true}
}
{value: v, ok} := parse("42")

// Rename to avoid collisions
{floor: mathFloor} := import("math")

// With defaults
{timeout: t = 5, retries: r = 3} := options

// Swap / combine with multi-value destructuring
a, b := 1, 2
{x, y} := {x: a + b, y: a - b}
```

## Interaction with existing features

- Compatible with multi-value function returns: both features are
  orthogonal syntactic sugar over the assignment statement.
- `import` already returns a map, so `{...} := import("mod")` is the
  natural spelling of the common idiom.
- Works with `assoc` / `dissoc` results since those also return maps.

## Implementation sketch

Parser changes:
- Extend `parseAssignStmt` to recognise `{` on the LHS of `:=` / `=`.
- Introduce a new AST node `MapDestructureExpr` (or reuse `MapLit` with a
  flag) that carries `(key, alias, default)` triples.

Compiler changes:
- Emit the RHS expression normally (produces a map on the stack).
- For each `(key, alias, default)` triple emit:
  - `OpConstant <key-string>` + `OpIndex` to extract the value, OR a new
    `OpGetField` opcode for efficiency.
  - If a default is provided, emit a conditional: if the result is
    `undefined`, replace it with the default value.
  - Emit the appropriate store opcode (`OpDefineLocal`, `OpSetGlobal`, etc.)
    for the alias variable.
- No new VM loop opcodes are strictly required (can reuse `OpIndex`), though
  a dedicated `OpGetField` would avoid repeated string-constant allocation.

Complexity estimate: moderate. Parser work is the bulk; VM changes are
minimal if `OpIndex` is reused.
