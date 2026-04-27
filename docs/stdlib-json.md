# Module - "json"

```golang
json := import("json")
```

## Functions

- `decode(b string/bytes) => (object, error)`: parses the JSON string and
  returns an object.
- `encode(o object) => (bytes, error)`: returns the JSON string (bytes) of the
  object. Unlike Go's JSON package, this function does not HTML-escape texts;
  use `html_escape` if needed.
- `indent(b string/bytes, prefix string, indent string) => (bytes, error)`:
  returns an indented form of input JSON bytes string.
- `html_escape(b string/bytes) => bytes`: returns an HTML-safe form of input
  JSON bytes string.

## Examples

```golang
json := import("json")

encoded, err := json.encode({a: 1, b: [2, 3, 4]})
if is_error(err) { /* handle */ }

indented, _ := json.indent(encoded, "", "  ")
html_safe := json.html_escape(encoded)

decoded, err := json.decode(encoded)   // {a: 1, b: [2, 3, 4]}
if is_error(err) { /* handle */ }
```
