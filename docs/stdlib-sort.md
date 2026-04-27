# Module - "sort"

```golang
sort := import("sort")
```

All functions return a new array and leave the original unchanged.

## Functions

- `ints(arr [int]) => [int]`: returns a sorted copy of an int array in
  ascending order.
- `floats(arr [float|int]) => [float]`: returns a sorted copy of a float array
  in ascending order. Integer elements are promoted to float.
- `strings(arr [string]) => [string]`: returns a sorted copy of a string array
  in ascending lexicographic order.
- `reverse(arr) => array`: returns a copy of the array with elements in reverse
  order. Works with any element type.
- `by(arr, fn) => array`: returns a sorted copy of the array using a comparator
  function. `fn(a, b)` must return `true` if `a` should come before `b`.
  Accepts both function literals and built-in callables.

## Examples

```go
sort := import("sort")

sort.ints([3, 1, 2])           // [1, 2, 3]
sort.floats([3.0, 1.5, 2.0])  // [1.5, 2.0, 3.0]
sort.strings(["b", "a", "c"]) // ["a", "b", "c"]
sort.reverse([1, 2, 3])        // [3, 2, 1]

words := ["banana", "apple", "cherry"]
sort.by(words, func(a, b) { return len(a) < len(b) })
// ["apple", "banana", "cherry"]
```
