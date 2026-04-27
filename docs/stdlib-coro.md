# Module - "coro"

```golang
coro := import("coro")
```

The `coro` module provides **coroutines**: functions that can pause themselves
mid-execution, hand a value back to their caller, and later be told to continue
from exactly where they stopped. All local variables and loop counters are
preserved across each pause.

The pause instruction is `yield`, which is passed as the **first argument** of
every coroutine function. This makes the dependency explicit in the function
signature rather than hiding it behind language magic.

## Functions

- `new(fn, args...) => coroutine`: creates a coroutine from a compiled
  function. `fn` must accept `yield` as its first parameter, followed by any
  additional `args` you want to pass in. The coroutine starts suspended; call
  `.resume()` to run it.

## Coroutine object

`coro.new` returns a coroutine object with the following members.

- `.resume() => value, bool`: resumes the coroutine. Returns the value passed
  to `yield` and `true` while the coroutine is alive. Returns `undefined` and
  `false` once the function has returned (the coroutine is dead). Calling
  `.resume()` on a dead coroutine is safe and always returns `undefined, false`.
- `.close()`: stops the coroutine. Any `yield` the coroutine is currently
  suspended at will unblock and the goroutine will exit. Calling `.close()` on a
  dead coroutine is a no-op.
- `.status`: a string property; `"suspended"` while the coroutine has more
  work to do, `"dead"` once it has finished or been closed.

Coroutine objects are **iterable**: they can be used directly in `for-in`
loops, which repeatedly calls `.resume()` behind the scenes until the coroutine
is dead.

---

## Basics

Define a generator by writing an ordinary function whose first parameter is
`yield`. Call `yield(value)` to pause and send a value to the caller.

```golang
coro := import("coro")

counter := func(yield, start) {
    for i := start; i < start+3; i++ {
        yield(i)
    }
}

co := coro.new(counter, 10)

v, ok := co.resume()   // v == 10, ok == true
v, ok  = co.resume()   // v == 11, ok == true
v, ok  = co.resume()   // v == 12, ok == true
v, ok  = co.resume()   // v == undefined, ok == false  (dead)
```

---

## Using coroutines with for-in

Because a coroutine is iterable, you can loop over it directly. This is the
cleanest pattern for pure generators.

```golang
coro := import("coro")

squares := func(yield, n) {
    for i := 1; i <= n; i++ {
        yield(i * i)
    }
}

for v in coro.new(squares, 5) {
    fmt.println(v)
}
// Output: 1  4  9  16  25
```

---

## Infinite sequences

Because the coroutine is only advanced when `.resume()` is called (or when the
for-in loop asks for the next value), a coroutine whose function contains an
infinite loop is perfectly fine. It only runs as far as the caller needs.

```golang
coro := import("coro")

naturals := func(yield) {
    n := 1
    for {
        yield(n)
        n++
    }
}

co := coro.new(naturals)

// Take only the first 5
count := 0
for v in co {
    fmt.println(v)
    count++
    if count == 5 { break }
}
// Output: 1  2  3  4  5

co.close()   // release the goroutine backing the coroutine
```

> **Always call `.close()`** when you stop consuming a coroutine before it
> finishes naturally. This releases the goroutine immediately rather than
> waiting for the garbage collector.

---

## Checking liveness

`.resume()` returns two values: the yielded value and a boolean that is `true`
while the coroutine is still alive. Use this when you cannot rely on a sentinel
value to detect the end of the sequence.

```golang
coro := import("coro")

words := func(yield) {
    yield("hello")
    yield("world")
}

co := coro.new(words)

for {
    word, ok := co.resume()
    if !ok { break }
    fmt.println(word)
}
```

---

## Multiple independent coroutines

Each call to `coro.new` creates a completely independent coroutine with its
own execution state. They do not share stack frames or local variables.

```golang
coro := import("coro")

make_counter := func(start) {
    return func(yield) {
        i := start
        for { yield(i); i++ }
    }
}

a := coro.new(make_counter(0))
b := coro.new(make_counter(100))

av, _ := a.resume()   // 0
bv, _ := b.resume()   // 100
av, _  = a.resume()   // 1
bv, _  = b.resume()   // 101

a.close()
b.close()
```

---

## Pipelines

Coroutines compose naturally into pipelines. Each stage reads from the previous
one by calling `.resume()`.

```golang
coro := import("coro")
fmt   := import("fmt")

// Stage 1: emit numbers 1..n
source := func(yield, n) {
    for i := 1; i <= n; i++ { yield(i) }
}

// Stage 2: keep only even numbers
evens := func(yield, upstream) {
    for {
        v, ok := upstream.resume()
        if !ok { break }
        if v % 2 == 0 { yield(v) }
    }
}

// Stage 3: square each value
squares := func(yield, upstream) {
    for {
        v, ok := upstream.resume()
        if !ok { break }
        yield(v * v)
    }
}

src := coro.new(source, 10)
evn := coro.new(evens, src)
sq  := coro.new(squares, evn)

for v in sq {
    fmt.println(v)
}
// Output: 4  16  36  64  100
// (squares of 2, 4, 6, 8, 10)

sq.close()
```

---

## Multi-stage pipeline: anomaly detection

A more realistic pipeline shows what coroutines add over plain function calls:
**each stage can be stateful**. The sliding-window stage below keeps its own
rolling buffer; the deviation stage tracks nothing itself; the anomaly stage
applies a threshold. None of them know about the others; they only speak
through coroutine handles.

The data flows lazily: nothing moves until the final consumer asks for the
next record.

```golang
coro := import("coro")
fmt  := import("fmt")

// Stage 1: raw sensor readings (index 7 is a spike)
source := func(yield) {
    data := [10, 11, 10, 12, 11, 10, 11, 45, 11, 10, 12, 11]
    for _, v in data {
        yield(v)
    }
}

// Stage 2: sliding-window average over the last n values
//   emits {raw, avg} maps; maintains its own rolling buffer
sliding_avg := func(yield, src, n) {
    win := []
    for {
        v, ok := src.resume()
        if !ok { break }
        win = append(win, v)
        if len(win) > n {
            win = win[1:]
        }
        sum := 0
        for _, w in win { sum += w }
        yield({raw: v, avg: sum / len(win)})
    }
}

// Stage 3: absolute deviation from the sliding average
//   emits {raw, avg, dev} maps; no internal state needed
deviation := func(yield, src) {
    for {
        rec, ok := src.resume()
        if !ok { break }
        d := rec.raw - rec.avg
        if d < 0 { d = -d }
        yield({raw: rec.raw, avg: rec.avg, dev: d})
    }
}

// Stage 4: suppress normal readings; only pass through anomalies
anomalies := func(yield, src, threshold) {
    for {
        rec, ok := src.resume()
        if !ok { break }
        if rec.dev > threshold {
            yield(rec)
        }
    }
}

src  := coro.new(source)
avg  := coro.new(sliding_avg, src, 4)
dev  := coro.new(deviation,  avg)
anom := coro.new(anomalies,  dev, 5)

for rec in anom {
    fmt.println("anomaly  raw:", rec.raw, " avg:", rec.avg, " dev:", rec.dev)
}
anom.close()
// Output:
// anomaly  raw:45 avg:19 dev:26
// anomaly  raw:11 avg:19 dev:8
// anomaly  raw:10 avg:19 dev:9
// anomaly  raw:12 avg:19 dev:7
//
// The spike at index 7 inflates the window average; the next three readings
// look anomalous because the average is still recovering from that spike.
```

---

## State machines

Coroutines are a natural fit for anything with sequential phases. The current
position in the function *is* the current state, with no explicit state variable
needed.

```golang
coro := import("coro")
fmt   := import("fmt")

// A simple connection-handshake simulator.
handshake := func(yield) {
    yield("SYN")
    yield("SYN-ACK")
    yield("ACK")
    yield("ESTABLISHED")
}

conn := coro.new(handshake)
for msg in conn {
    fmt.println(">>", msg)
}
// Output:
// >> SYN
// >> SYN-ACK
// >> ACK
// >> ESTABLISHED
```

---

## Error handling

If the coroutine function causes a runtime error, it propagates back to the
caller at the next `.resume()` call, just like any other runtime error.

```golang
coro := import("coro")

bad := func(yield) {
    yield(1)
    x := 5
    x()         // runtime error: not callable
}

co := coro.new(bad)
co.resume()     // returns 1, true (fine so far)
co.resume()     // runtime error surfaces here
```

---

## The yield function

`yield` is an ordinary function value. You can store it, pass it deeper, or
call it from helper functions, whatever the code requires.

```golang
coro := import("coro")

emit_range := func(yield, lo, hi) {
    for i := lo; i <= hi; i++ {
        yield(i)
    }
}

walk_tree := func(yield, tree) {
    // tree is {val: int, left: map/undefined, right: map/undefined}
    if is_undefined(tree) { return }
    walk_tree(yield, tree.left)
    yield(tree.val)
    walk_tree(yield, tree.right)
}

tree := {val: 4,
    left:  {val: 2, left: {val: 1, left: undefined, right: undefined},
                    right: {val: 3, left: undefined, right: undefined}},
    right: {val: 5, left: undefined, right: undefined}}

for v in coro.new(walk_tree, tree) {
    fmt.println(v)   // in-order: 1 2 3 4 5
}
```

---

## Concurrency note

Each coroutine runs in its own Go goroutine, but only one side runs at a
time: the parent is always blocked waiting for a yield while the coroutine
runs, and vice versa. This means shared global variables are safe to read and
write from a coroutine without additional locking, as long as you do not run
the same `Compiled` script concurrently from multiple Go goroutines.

Each coroutine's goroutine holds memory for as long as it is alive. Always
call `.close()` when you are done with a coroutine that has not finished
naturally.
