# Module - "rand"

```golang
rand := import("rand")
```

The module-level functions draw from a shared, unseeded source. For
reproducible sequences or isolated state, use `rand.new(seed)`.

## Functions

- `new() => Rand`: returns a new isolated Rand instance seeded randomly.
- `new(seed int) => Rand`: returns a new isolated Rand instance seeded
  deterministically. Use this for reproducible sequences or when inter-script
  isolation matters.
- `exp_float() => float`: returns an exponentially distributed float64 in the
  range (0, +math.MaxFloat64] with an exponential distribution whose rate
  parameter (lambda) is 1 and whose mean is 1/lambda (1).
- `float() => float`: returns a pseudo-random float64 in [0.0,1.0).
- `int() => int`: returns a non-negative pseudo-random 63-bit integer as int64.
- `intn(n int) => int`: returns a non-negative pseudo-random int64 in [0,n).
- `norm_float() => float`: returns a normally distributed float64 in the range
  [-math.MaxFloat64, +math.MaxFloat64] with standard normal distribution
  (mean = 0, stddev = 1).
- `perm(n int) => [int]`: returns a pseudo-random permutation of the integers
  [0,n).
- `read(p bytes) => int`: generates len(p) random bytes and writes them into p.
  Always returns len(p).

## Rand

Returned by `rand.new()`. Has its own independent source; `seed()` only affects
this instance.

- `seed(seed int)`: resets this instance to a deterministic state.
- `exp_float() => float`: same as module-level, from this instance's source.
- `float() => float`: same as module-level, from this instance's source.
- `int() => int`: same as module-level, from this instance's source.
- `intn(n int) => int`: same as module-level, from this instance's source.
- `norm_float() => float`: same as module-level, from this instance's source.
- `perm(n int) => [int]`: same as module-level, from this instance's source.
- `read(p bytes) => int`: same as module-level, from this instance's source.
