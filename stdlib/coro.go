package stdlib

import (
	"context"
	"errors"
	"fmt"
	"sync"

	tengo "github.com/tengolang/tengo/v3"
	"github.com/tengolang/tengo/v3/parser"
)

// errCoroClosed is the internal signal propagated when a coroutine is closed
// via Close(). It is never surfaced to Tengo callers.
var errCoroClosed = errors.New("coroutine closed")

// emptyMainFn is the synthetic base frame used by child VMs. It is never
// actually executed; RunCompiledFunction returns before the frame is popped
// back to this one.
var emptyMainFn = &tengo.CompiledFunction{
	Instructions: []byte{byte(parser.OpSuspend)},
}

// coroutineModule is the "coro" stdlib module.
var coroutineModule = map[string]tengo.Object{
	// new(fn, args...) creates a coroutine from a compiled function.
	// fn must accept yield as its first parameter, followed by any user args.
	// The coroutine starts suspended; call .resume() to run it.
	"new": &tengo.InteropFunction{
		Name: "new",
		Value: func(v *tengo.VM, args ...tengo.Object) (tengo.Object, error) {
			if len(args) < 1 {
				return nil, tengo.ErrWrongNumArguments
			}
			fn, ok := args[0].(*tengo.CompiledFunction)
			if !ok {
				return nil, tengo.ErrInvalidArgumentType{
					Name:     "first",
					Expected: "compiled-function",
					Found:    args[0].TypeName(),
				}
			}
			if fn.VarArgs {
				return nil, fmt.Errorf("coroutine function cannot be variadic")
			}
			return newCoroutine(v, fn, args[1:])
		},
	},
}

// Coroutine is the Tengo object type for coroutines.
// It is goroutine-backed: each coroutine runs in its own goroutine and
// communicates with the caller via channels. At most one side runs at
// a time, so shared globals remain consistent without additional locking.
//
// resume() is not safe for concurrent use. Call it from one goroutine at
// a time, which is the normal usage pattern.
type Coroutine struct {
	tengo.ObjectImpl

	// toCaller carries values yielded by the coroutine goroutine.
	// It is closed (never sent on) when the goroutine exits.
	toCaller chan tengo.Object

	// toCoro is the resume signal sent by the caller to unblock a
	// suspended yield. Buffered(1) so the sender never blocks even if
	// the coroutine has already exited.
	toCoro chan struct{}

	cancel context.CancelFunc

	mu      sync.Mutex
	started bool
	dead    bool
	err     error
}

func newCoroutine(parentVM *tengo.VM, fn *tengo.CompiledFunction, fnArgs []tengo.Object) (*Coroutine, error) {
	// fn.NumParameters includes the injected yield param, so the caller must
	// supply exactly fn.NumParameters-1 user arguments.
	want := fn.NumParameters - 1
	if len(fnArgs) != want {
		return nil, fmt.Errorf(
			"wrong number of arguments: coroutine function needs %d user arg(s), got %d",
			want, len(fnArgs))
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Coroutine{
		toCaller: make(chan tengo.Object),
		toCoro:   make(chan struct{}, 1),
		cancel:   cancel,
	}

	// Capture channel pointers for the yield closure (avoids holding a
	// reference to c inside the closure, breaking the GC cycle).
	toCaller := c.toCaller
	toCoro := c.toCoro

	// yield is injected as the first argument of fn.
	// Calling yield(val) sends val to the caller and suspends until the
	// next resume(). Returns undefined on resume; returns errCoroClosed
	// when the coroutine has been closed (which stops the VM).
	yieldFn := &tengo.UserFunction{
		Name: "yield",
		Value: func(args ...tengo.Object) (tengo.Object, error) {
			var val tengo.Object = tengo.UndefinedValue
			if len(args) >= 1 {
				val = args[0]
			}
			select {
			case toCaller <- val:
			case <-ctx.Done():
				return nil, errCoroClosed
			}
			select {
			case <-toCoro:
				return tengo.UndefinedValue, nil
			case <-ctx.Done():
				return nil, errCoroClosed
			}
		},
	}

	allArgs := make([]tengo.Object, 0, 1+len(fnArgs))
	allArgs = append(allArgs, yieldFn)
	allArgs = append(allArgs, fnArgs...)

	// Build a child VM that shares constants, file-set, and globals with the
	// parent. Constants and file-set are read-only. Globals are safe to share
	// because the parent VM is always blocked on a channel while the child runs.
	childBytecode := &tengo.Bytecode{
		MainFunction: emptyMainFn,
		Constants:    parentVM.Constants(),
		FileSet:      parentVM.SourceFileSet(),
	}
	childVM := tengo.NewVM(childBytecode, parentVM.VMGlobals(), -1)

	go func() {
		// Always close toCaller when the goroutine exits so any blocked
		// resume() call unblocks immediately.
		defer close(c.toCaller)
		defer func() {
			if r := recover(); r != nil {
				c.mu.Lock()
				switch e := r.(type) {
				case error:
					c.err = e
				default:
					c.err = fmt.Errorf("coroutine panic: %v", e)
				}
				c.mu.Unlock()
			}
		}()

		_, err := childVM.RunCompiledFunction(fn, allArgs...)
		if err != nil && !errors.Is(err, errCoroClosed) {
			c.mu.Lock()
			c.err = err
			c.mu.Unlock()
		}
	}()

	return c, nil
}

// TypeName implements tengo.Object.
func (c *Coroutine) TypeName() string { return "coroutine" }

// String implements tengo.Object.
func (c *Coroutine) String() string {
	return "coroutine(" + c.statusString() + ")"
}

// CanIterate implements tengo.Object — coroutines can be used with for-in.
func (c *Coroutine) CanIterate() bool { return true }

// Iterate implements tengo.Object — returns an iterator backed by resume().
func (c *Coroutine) Iterate() tengo.Iterator {
	return &coroutineIter{co: c}
}

// IndexGet implements tengo.Object — exposes .resume(), .close(), .status.
func (c *Coroutine) IndexGet(index tengo.Object) (tengo.Object, error) {
	sv, ok := tengo.StringValue(index)
	if !ok {
		return tengo.UndefinedValue, nil
	}
	switch sv {
	case "resume":
		return &tengo.UserFunction{
			Name: "resume",
			Value: func(_ ...tengo.Object) (tengo.Object, error) {
				val, alive, err := c.internalResume()
				if err != nil {
					return nil, err
				}
				if val == nil {
					val = tengo.UndefinedValue
				}
				var aliveObj tengo.Object
				if alive {
					aliveObj = tengo.TrueValue
				} else {
					aliveObj = tengo.FalseValue
				}
				return &tengo.MultiValue{Values: []tengo.Object{val, aliveObj}}, nil
			},
		}, nil
	case "close":
		return &tengo.UserFunction{
			Name: "close",
			Value: func(_ ...tengo.Object) (tengo.Object, error) {
				c.doClose()
				return tengo.UndefinedValue, nil
			},
		}, nil
	case "status":
		return &tengo.String{Value: c.statusString()}, nil
	}
	return tengo.UndefinedValue, nil
}

// internalResume drives one step of the coroutine.
// Returns (yieldedValue, alive, error).
// alive=false means the coroutine has finished; error is non-nil only on
// an unhandled error inside the coroutine body.
func (c *Coroutine) internalResume() (tengo.Object, bool, error) {
	c.mu.Lock()
	if c.dead {
		err := c.err
		c.mu.Unlock()
		return nil, false, err
	}
	started := c.started
	c.started = true
	c.mu.Unlock()

	if started {
		// Unblock the suspended yield. The channel is buffered(1) so this
		// never blocks, even if the coroutine has already exited.
		c.toCoro <- struct{}{}
	}

	val, ok := <-c.toCaller
	if !ok {
		// Channel closed: goroutine has exited.
		c.mu.Lock()
		c.dead = true
		err := c.err
		c.mu.Unlock()
		return nil, false, err
	}
	return val, true, nil
}

func (c *Coroutine) doClose() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.dead {
		c.cancel()
	}
}

func (c *Coroutine) statusString() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dead {
		return "dead"
	}
	return "suspended"
}

// coroutineIter adapts Coroutine to tengo.Iterator for for-in loop support.
type coroutineIter struct {
	tengo.ObjectImpl
	co  *Coroutine
	cur tengo.Object
}

func (i *coroutineIter) TypeName() string { return "coroutine-iterator" }
func (i *coroutineIter) String() string   { return "<coroutine-iterator>" }

func (i *coroutineIter) Next() bool {
	val, alive, err := i.co.internalResume()
	if err != nil || !alive {
		return false
	}
	if val == nil {
		i.cur = tengo.UndefinedValue
	} else {
		i.cur = val
	}
	return true
}

func (i *coroutineIter) Key() tengo.Object { return tengo.UndefinedValue }

func (i *coroutineIter) Value() tengo.Object {
	if i.cur == nil {
		return tengo.UndefinedValue
	}
	return i.cur
}
