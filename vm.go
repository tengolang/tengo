package tengo

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/tengolang/tengo/v3/parser"
	"github.com/tengolang/tengo/v3/token"
)

// frame represents a function call frame.
type frame struct {
	fn          *CompiledFunction
	freeVars    []*ObjectPtr
	ip          int
	basePointer int
}

// stopping bit-flags stored in VM.stopping (single atomic word).
const (
	stoppingAbort int64 = 1 // set by Abort()
	stoppingPause int64 = 2 // set by Pause(), cleared by Resume() and Run()
)

// VM is a virtual machine that executes the bytecode compiled by Compiler.
type VM struct {
	constants   []Object
	stack       [StackSize]Object
	sp          int
	globals     []Object
	fileSet     *parser.SourceFileSet
	frames      [MaxFrames]frame
	framesIndex int
	curFrame    *frame
	curInsts    []byte
	ip          int
	stopping    int64 // atomic bit-field: stoppingAbort | stoppingPause
	maxAllocs   int64
	allocs      int64
	err         error
	resumeDepth int      // non-zero: run() stops when framesIndex drops to this value
	hookFunc    HookFunc // nil when tracing is disabled
	hookMask    HookMask // bitmask of enabled events
	lastLine    int      // last seen source line, for HookLine dedup
}

// NewVM creates a VM.
func NewVM(
	bytecode *Bytecode,
	globals []Object,
	maxAllocs int64,
) *VM {
	if globals == nil {
		globals = make([]Object, GlobalsSize)
	}
	v := &VM{
		constants:   bytecode.Constants,
		sp:          0,
		globals:     globals,
		fileSet:     bytecode.FileSet,
		framesIndex: 1,
		ip:          -1,
		maxAllocs:   maxAllocs,
	}
	v.frames[0].fn = bytecode.MainFunction
	v.frames[0].ip = -1
	v.curFrame = &v.frames[0]
	v.curInsts = v.curFrame.fn.Instructions
	return v
}

// Abort aborts the execution. Safe to call from any goroutine.
func (v *VM) Abort() {
	atomic.StoreInt64(&v.stopping, stoppingAbort)
}

// SetHook installs a hook function that the VM calls at the events selected
// by mask. Pass fn=nil (or mask=0) to disable tracing.
func (v *VM) SetHook(fn HookFunc, mask HookMask) {
	v.hookFunc = fn
	v.hookMask = mask
	v.lastLine = 0
}

// Pause suspends execution after the current instruction completes.
// Safe to call from any goroutine. Call Resume to continue from the same point.
func (v *VM) Pause() {
	atomic.StoreInt64(&v.stopping, stoppingPause)
}

// IsPaused reports whether the VM is currently paused.
func (v *VM) IsPaused() bool {
	return atomic.LoadInt64(&v.stopping)&stoppingPause != 0
}

// Resume continues execution after a Pause. Must be called from the goroutine
// that owns the VM — i.e. after Run() or a previous Resume() has returned.
// Returns nil when the script finishes normally; check IsPaused() to
// distinguish a normal finish from another Pause.
func (v *VM) Resume() error {
	atomic.StoreInt64(&v.stopping, 0)
	v.run()
	err := v.err
	if err != nil {
		filePos := v.fileSet.Position(
			v.curFrame.fn.SourcePos(v.ip - 1))
		err = fmt.Errorf("runtime error: %w\n\tat %s", err, filePos)
		for v.framesIndex > 1 {
			v.framesIndex--
			v.curFrame = &v.frames[v.framesIndex-1]
			filePos = v.fileSet.Position(
				v.curFrame.fn.SourcePos(v.curFrame.ip - 1))
			err = fmt.Errorf("%w\n\tat %s", err, filePos)
		}
		return err
	}
	return nil
}

// RunContext is like Run but stops the VM when ctx is cancelled.
func (v *VM) RunContext(ctx context.Context) error {
	ch := make(chan error, 1)
	go func() { ch <- v.Run() }()
	select {
	case <-ctx.Done():
		v.Abort()
		<-ch
		return ctx.Err()
	case err := <-ch:
		return err
	}
}

// Run starts the execution.
func (v *VM) Run() (err error) {
	// reset VM states
	v.sp = 0
	v.curFrame = &(v.frames[0])
	v.curInsts = v.curFrame.fn.Instructions
	v.framesIndex = 1
	v.ip = -1
	v.allocs = v.maxAllocs + 1
	atomic.StoreInt64(&v.stopping, 0)

	v.run()
	err = v.err
	if err != nil {
		filePos := v.fileSet.Position(
			v.curFrame.fn.SourcePos(v.ip - 1))
		err = fmt.Errorf("runtime error: %w\n\tat %s",
			err, filePos)
		for v.framesIndex > 1 {
			v.framesIndex--
			v.curFrame = &v.frames[v.framesIndex-1]
			filePos = v.fileSet.Position(
				v.curFrame.fn.SourcePos(v.curFrame.ip - 1))
			err = fmt.Errorf("%w\n\tat %s", err, filePos)
		}
		return err
	}
	return nil
}

func (v *VM) run() {
	// Hoist hot fields to locals so the compiler can keep them in registers
	// across the dispatch loop. Flush back to the struct at every exit point
	// and at frame switches (OpCall/OpReturn).
	ip := v.ip
	sp := v.sp
	curInsts := v.curInsts
	bp := v.curFrame.basePointer
	hookMask := v.hookMask

	for {
		ip++

		if hookMask&HookMaskLine != 0 {
			pos := v.fileSet.Position(v.curFrame.fn.SourcePos(ip))
			if pos.Line != v.lastLine {
				v.lastLine = pos.Line
				v.hookFunc(v, HookInfo{
					Event: HookLine,
					Depth: v.framesIndex,
					Pos:   pos,
				})
				hookMask = v.hookMask
				if atomic.LoadInt64(&v.stopping) != 0 {
					// ip points to the opcode of the unexecuted instruction;
					// save ip-1 so the loop's ip++ at restart lands on the opcode.
					v.ip = ip - 1
					v.sp = sp
					return
				}
			}
		}

		switch curInsts[ip] {
		case parser.OpConstant:
			ip += 2
			cidx := int(curInsts[ip]) | int(curInsts[ip-1])<<8

			v.stack[sp] = v.constants[cidx]
			sp++
		case parser.OpNull:
			v.stack[sp] = UndefinedValue
			sp++
		case parser.OpBinaryOp:
			ip++
			right := v.stack[sp-1]
			left := v.stack[sp-2]
			tok := token.Token(curInsts[ip])
			// Fast path: Int op Int avoids virtual dispatch and type switches.
			if li, ok := left.(Int); ok {
				if ri, ok := right.(Int); ok {
					var r int64
					switch tok {
					case token.Add:
						r = li.Value + ri.Value
					case token.Sub:
						r = li.Value - ri.Value
					case token.Mul:
						r = li.Value * ri.Value
					case token.Less:
						v.allocs--
						if v.allocs == 0 {
							v.err = ErrObjectAllocLimit
							v.ip = ip
							v.sp = sp - 2
							return
						}
						if li.Value < ri.Value {
							v.stack[sp-2] = TrueValue
						} else {
							v.stack[sp-2] = FalseValue
						}
						sp--
						continue
					case token.Greater:
						v.allocs--
						if v.allocs == 0 {
							v.err = ErrObjectAllocLimit
							v.ip = ip
							v.sp = sp - 2
							return
						}
						if li.Value > ri.Value {
							v.stack[sp-2] = TrueValue
						} else {
							v.stack[sp-2] = FalseValue
						}
						sp--
						continue
					case token.LessEq:
						v.allocs--
						if v.allocs == 0 {
							v.err = ErrObjectAllocLimit
							v.ip = ip
							v.sp = sp - 2
							return
						}
						if li.Value <= ri.Value {
							v.stack[sp-2] = TrueValue
						} else {
							v.stack[sp-2] = FalseValue
						}
						sp--
						continue
					case token.GreaterEq:
						v.allocs--
						if v.allocs == 0 {
							v.err = ErrObjectAllocLimit
							v.ip = ip
							v.sp = sp - 2
							return
						}
						if li.Value >= ri.Value {
							v.stack[sp-2] = TrueValue
						} else {
							v.stack[sp-2] = FalseValue
						}
						sp--
						continue
					default:
						goto slowBinaryOp
					}
					v.allocs--
					if v.allocs == 0 {
						v.err = ErrObjectAllocLimit
						v.ip = ip
						v.sp = sp - 2
						return
					}
					v.stack[sp-2] = Int{Value: r}
					sp--
					continue
				}
			}
		slowBinaryOp:
			res, e := left.BinaryOp(tok, right)
			if e != nil {
				sp -= 2
				if e == ErrInvalidOperator {
					v.err = fmt.Errorf("invalid operation: %s %s %s",
						left.TypeName(), tok.String(), right.TypeName())
					v.ip = ip
					v.sp = sp
					return
				}
				v.err = e
				v.ip = ip
				v.sp = sp
				return
			}

			v.allocs--
			if v.allocs == 0 {
				v.err = ErrObjectAllocLimit
				v.ip = ip
				v.sp = sp
				return
			}

			v.stack[sp-2] = res
			sp--
		case parser.OpEqual:
			right := v.stack[sp-1]
			left := v.stack[sp-2]
			sp -= 2
			if li, lok := left.(Int); lok {
				if ri, rok := right.(Int); rok {
					if li.Value == ri.Value {
						v.stack[sp] = TrueValue
					} else {
						v.stack[sp] = FalseValue
					}
					sp++
					continue
				}
			}
			if left.Equals(right) {
				v.stack[sp] = TrueValue
			} else {
				v.stack[sp] = FalseValue
			}
			sp++
		case parser.OpNotEqual:
			right := v.stack[sp-1]
			left := v.stack[sp-2]
			sp -= 2
			if li, lok := left.(Int); lok {
				if ri, rok := right.(Int); rok {
					if li.Value != ri.Value {
						v.stack[sp] = TrueValue
					} else {
						v.stack[sp] = FalseValue
					}
					sp++
					continue
				}
			}
			if left.Equals(right) {
				v.stack[sp] = FalseValue
			} else {
				v.stack[sp] = TrueValue
			}
			sp++
		case parser.OpPop:
			sp--
		case parser.OpTrue:
			v.stack[sp] = TrueValue
			sp++
		case parser.OpFalse:
			v.stack[sp] = FalseValue
			sp++
		case parser.OpLNot:
			operand := v.stack[sp-1]
			sp--
			if operand.IsFalsy() {
				v.stack[sp] = TrueValue
			} else {
				v.stack[sp] = FalseValue
			}
			sp++
		case parser.OpBComplement:
			operand := v.stack[sp-1]
			sp--

			switch x := operand.(type) {
			case Int:
				var res Object = Int{Value: ^x.Value}
				v.allocs--
				if v.allocs == 0 {
					v.err = ErrObjectAllocLimit
					v.ip = ip
					v.sp = sp
					return
				}
				v.stack[sp] = res
				sp++
			case *Bytes:
				res := make([]byte, len(x.Value))
				for i, b := range x.Value {
					res[i] = ^b
				}
				var obj Object = &Bytes{Value: res}
				v.allocs--
				if v.allocs == 0 {
					v.err = ErrObjectAllocLimit
					v.ip = ip
					v.sp = sp
					return
				}
				v.stack[sp] = obj
				sp++
			default:
				v.err = fmt.Errorf("invalid operation: ^%s",
					operand.TypeName())
				v.ip = ip
				v.sp = sp
				return
			}
		case parser.OpMinus:
			operand := v.stack[sp-1]
			sp--

			switch x := operand.(type) {
			case Int:
				var res Object = Int{Value: -x.Value}
				v.allocs--
				if v.allocs == 0 {
					v.err = ErrObjectAllocLimit
					v.ip = ip
					v.sp = sp
					return
				}
				v.stack[sp] = res
				sp++
			case Float:
				var res Object = Float{Value: -x.Value}
				v.allocs--
				if v.allocs == 0 {
					v.err = ErrObjectAllocLimit
					v.ip = ip
					v.sp = sp
					return
				}
				v.stack[sp] = res
				sp++
			default:
				v.err = fmt.Errorf("invalid operation: -%s",
					operand.TypeName())
				v.ip = ip
				v.sp = sp
				return
			}
		case parser.OpJumpFalsy:
			ip += 4
			sp--
			if v.stack[sp].IsFalsy() {
				pos := int(curInsts[ip]) | int(curInsts[ip-1])<<8 | int(curInsts[ip-2])<<16 | int(curInsts[ip-3])<<24
				ip = pos - 1
			}
		case parser.OpAndJump:
			ip += 4
			if v.stack[sp-1].IsFalsy() {
				pos := int(curInsts[ip]) | int(curInsts[ip-1])<<8 | int(curInsts[ip-2])<<16 | int(curInsts[ip-3])<<24
				ip = pos - 1
			} else {
				sp--
			}
		case parser.OpOrJump:
			ip += 4
			if v.stack[sp-1].IsFalsy() {
				sp--
			} else {
				pos := int(curInsts[ip]) | int(curInsts[ip-1])<<8 | int(curInsts[ip-2])<<16 | int(curInsts[ip-3])<<24
				ip = pos - 1
			}
		case parser.OpJump:
			pos := int(curInsts[ip+4]) | int(curInsts[ip+3])<<8 | int(curInsts[ip+2])<<16 | int(curInsts[ip+1])<<24
			if pos <= ip && atomic.LoadInt64(&v.stopping) != 0 {
				return
			}
			ip = pos - 1
		case parser.OpSetGlobal:
			ip += 2
			sp--
			globalIndex := int(curInsts[ip]) | int(curInsts[ip-1])<<8
			v.globals[globalIndex] = unwrapMultiValue(v.stack[sp])
		case parser.OpSetSelGlobal:
			ip += 3
			globalIndex := int(curInsts[ip-1]) | int(curInsts[ip-2])<<8
			numSelectors := int(curInsts[ip])

			// selectors and RHS value
			selectors := make([]Object, numSelectors)
			for i := 0; i < numSelectors; i++ {
				selectors[i] = v.stack[sp-numSelectors+i]
			}
			val := v.stack[sp-numSelectors-1]
			sp -= numSelectors + 1
			e := indexAssign(v.globals[globalIndex], val, selectors)
			if e != nil {
				v.err = e
				v.ip = ip
				v.sp = sp
				return
			}
		case parser.OpGetGlobal:
			ip += 2
			globalIndex := int(curInsts[ip]) | int(curInsts[ip-1])<<8
			val := v.globals[globalIndex]
			v.stack[sp] = val
			sp++
		case parser.OpArray:
			ip += 2
			numElements := int(curInsts[ip]) | int(curInsts[ip-1])<<8

			var elements []Object
			for i := sp - numElements; i < sp; i++ {
				elements = append(elements, v.stack[i])
			}
			sp -= numElements

			var arr Object = &Array{Value: elements}
			v.allocs--
			if v.allocs == 0 {
				v.err = ErrObjectAllocLimit
				v.ip = ip
				v.sp = sp
				return
			}

			v.stack[sp] = arr
			sp++
		case parser.OpMap:
			ip += 2
			numElements := int(curInsts[ip]) | int(curInsts[ip-1])<<8
			kv := make(map[string]Object, numElements)
			for i := sp - numElements; i < sp; i += 2 {
				key := v.stack[i]
				value := v.stack[i+1]
				kv[key.(*String).Value] = value
			}
			sp -= numElements

			var m Object = &Map{Value: kv}
			v.allocs--
			if v.allocs == 0 {
				v.err = ErrObjectAllocLimit
				v.ip = ip
				v.sp = sp
				return
			}
			v.stack[sp] = m
			sp++
		case parser.OpError:
			value := v.stack[sp-1]
			var e Object = &Error{
				Value: value,
			}
			v.allocs--
			if v.allocs == 0 {
				v.err = ErrObjectAllocLimit
				v.ip = ip
				v.sp = sp
				return
			}
			v.stack[sp-1] = e
		case parser.OpImmutable:
			value := v.stack[sp-1]
			switch value := value.(type) {
			case *Array:
				var immutableArray Object = &ImmutableArray{
					Value: value.Value,
				}
				v.allocs--
				if v.allocs == 0 {
					v.err = ErrObjectAllocLimit
					v.ip = ip
					v.sp = sp
					return
				}
				v.stack[sp-1] = immutableArray
			case *Map:
				var immutableMap Object = &ImmutableMap{
					Value: value.Value,
				}
				v.allocs--
				if v.allocs == 0 {
					v.err = ErrObjectAllocLimit
					v.ip = ip
					v.sp = sp
					return
				}
				v.stack[sp-1] = immutableMap
			}
		case parser.OpIndex:
			index := v.stack[sp-1]
			left := v.stack[sp-2]
			sp -= 2

			val, err := left.IndexGet(index)
			if err != nil {
				if err == ErrNotIndexable {
					v.err = fmt.Errorf("not indexable: %s", index.TypeName())
					v.ip = ip
					v.sp = sp
					return
				}
				if err == ErrInvalidIndexType {
					v.err = fmt.Errorf("invalid index type: %s",
						index.TypeName())
					v.ip = ip
					v.sp = sp
					return
				}
				v.err = err
				v.ip = ip
				v.sp = sp
				return
			}
			if val == nil {
				val = UndefinedValue
			}
			v.stack[sp] = val
			sp++
		case parser.OpSliceIndex:
			high := v.stack[sp-1]
			low := v.stack[sp-2]
			left := v.stack[sp-3]
			sp -= 3

			var lowIdx int64
			if low != UndefinedValue {
				if lowInt, ok := low.(Int); ok {
					lowIdx = lowInt.Value
				} else {
					v.err = fmt.Errorf("invalid slice index type: %s",
						low.TypeName())
					v.ip = ip
					v.sp = sp
					return
				}
			}

			switch left := left.(type) {
			case *Array:
				numElements := int64(len(left.Value))
				var highIdx int64
				if high == UndefinedValue {
					highIdx = numElements
				} else if highInt, ok := high.(Int); ok {
					highIdx = highInt.Value
				} else {
					v.err = fmt.Errorf("invalid slice index type: %s",
						high.TypeName())
					v.ip = ip
					v.sp = sp
					return
				}
				if lowIdx > highIdx {
					v.err = fmt.Errorf("invalid slice index: %d > %d",
						lowIdx, highIdx)
					v.ip = ip
					v.sp = sp
					return
				}
				if lowIdx < 0 {
					lowIdx = 0
				} else if lowIdx > numElements {
					lowIdx = numElements
				}
				if highIdx < 0 {
					highIdx = 0
				} else if highIdx > numElements {
					highIdx = numElements
				}
				var val Object = &Array{
					Value: left.Value[lowIdx:highIdx],
				}
				v.allocs--
				if v.allocs == 0 {
					v.err = ErrObjectAllocLimit
					v.ip = ip
					v.sp = sp
					return
				}
				v.stack[sp] = val
				sp++
			case *ImmutableArray:
				numElements := int64(len(left.Value))
				var highIdx int64
				if high == UndefinedValue {
					highIdx = numElements
				} else if highInt, ok := high.(Int); ok {
					highIdx = highInt.Value
				} else {
					v.err = fmt.Errorf("invalid slice index type: %s",
						high.TypeName())
					v.ip = ip
					v.sp = sp
					return
				}
				if lowIdx > highIdx {
					v.err = fmt.Errorf("invalid slice index: %d > %d",
						lowIdx, highIdx)
					v.ip = ip
					v.sp = sp
					return
				}
				if lowIdx < 0 {
					lowIdx = 0
				} else if lowIdx > numElements {
					lowIdx = numElements
				}
				if highIdx < 0 {
					highIdx = 0
				} else if highIdx > numElements {
					highIdx = numElements
				}
				var val Object = &Array{
					Value: left.Value[lowIdx:highIdx],
				}
				v.allocs--
				if v.allocs == 0 {
					v.err = ErrObjectAllocLimit
					v.ip = ip
					v.sp = sp
					return
				}
				v.stack[sp] = val
				sp++
			case *String:
				numElements := int64(len(left.Value))
				var highIdx int64
				if high == UndefinedValue {
					highIdx = numElements
				} else if highInt, ok := high.(Int); ok {
					highIdx = highInt.Value
				} else {
					v.err = fmt.Errorf("invalid slice index type: %s",
						high.TypeName())
					v.ip = ip
					v.sp = sp
					return
				}
				if lowIdx > highIdx {
					v.err = fmt.Errorf("invalid slice index: %d > %d",
						lowIdx, highIdx)
					v.ip = ip
					v.sp = sp
					return
				}
				if lowIdx < 0 {
					lowIdx = 0
				} else if lowIdx > numElements {
					lowIdx = numElements
				}
				if highIdx < 0 {
					highIdx = 0
				} else if highIdx > numElements {
					highIdx = numElements
				}
				var val Object = &String{
					Value: left.Value[lowIdx:highIdx],
				}
				v.allocs--
				if v.allocs == 0 {
					v.err = ErrObjectAllocLimit
					v.ip = ip
					v.sp = sp
					return
				}
				v.stack[sp] = val
				sp++
			case *StringBuilder:
				mat := left.materialize()
				numElements := int64(len(mat.Value))
				var highIdx int64
				if high == UndefinedValue {
					highIdx = numElements
				} else if highInt, ok := high.(Int); ok {
					highIdx = highInt.Value
				} else {
					v.err = fmt.Errorf("invalid slice index type: %s",
						high.TypeName())
					v.ip = ip
					v.sp = sp
					return
				}
				if lowIdx > highIdx {
					v.err = fmt.Errorf("invalid slice index: %d > %d",
						lowIdx, highIdx)
					v.ip = ip
					v.sp = sp
					return
				}
				if lowIdx < 0 {
					lowIdx = 0
				} else if lowIdx > numElements {
					lowIdx = numElements
				}
				if highIdx < 0 {
					highIdx = 0
				} else if highIdx > numElements {
					highIdx = numElements
				}
				var val Object = &String{
					Value: mat.Value[lowIdx:highIdx],
				}
				v.allocs--
				if v.allocs == 0 {
					v.err = ErrObjectAllocLimit
					v.ip = ip
					v.sp = sp
					return
				}
				v.stack[sp] = val
				sp++
			case *Bytes:
				numElements := int64(len(left.Value))
				var highIdx int64
				if high == UndefinedValue {
					highIdx = numElements
				} else if highInt, ok := high.(Int); ok {
					highIdx = highInt.Value
				} else {
					v.err = fmt.Errorf("invalid slice index type: %s",
						high.TypeName())
					v.ip = ip
					v.sp = sp
					return
				}
				if lowIdx > highIdx {
					v.err = fmt.Errorf("invalid slice index: %d > %d",
						lowIdx, highIdx)
					v.ip = ip
					v.sp = sp
					return
				}
				if lowIdx < 0 {
					lowIdx = 0
				} else if lowIdx > numElements {
					lowIdx = numElements
				}
				if highIdx < 0 {
					highIdx = 0
				} else if highIdx > numElements {
					highIdx = numElements
				}
				var val Object = &Bytes{
					Value: left.Value[lowIdx:highIdx],
				}
				v.allocs--
				if v.allocs == 0 {
					v.err = ErrObjectAllocLimit
					v.ip = ip
					v.sp = sp
					return
				}
				v.stack[sp] = val
				sp++
			default:
				v.err = fmt.Errorf("not indexable: %s", left.TypeName())
				v.ip = ip
				v.sp = sp
				return
			}
		case parser.OpCall:
			numArgs := int(curInsts[ip+1])
			spread := int(curInsts[ip+2])
			ip += 2

			value := v.stack[sp-1-numArgs]

			if spread == 1 {
				sp--
				switch arr := v.stack[sp].(type) {
				case *Array:
					for _, item := range arr.Value {
						v.stack[sp] = item
						sp++
					}
					numArgs += len(arr.Value) - 1
				case *ImmutableArray:
					for _, item := range arr.Value {
						v.stack[sp] = item
						sp++
					}
					numArgs += len(arr.Value) - 1
				default:
					v.err = fmt.Errorf("not an array: %s", arr.TypeName())
					v.ip = ip
					v.sp = sp
					return
				}
			}

			if callee, ok := value.(*CompiledFunction); ok {
				if callee.VarArgs {
					// if the closure is variadic,
					// roll up all variadic parameters into an array
					realArgs := callee.NumParameters - 1
					varArgs := numArgs - realArgs
					if varArgs >= 0 {
						numArgs = realArgs + 1
						args := make([]Object, varArgs)
						spStart := sp - varArgs
						for i := spStart; i < sp; i++ {
							args[i-spStart] = v.stack[i]
						}
						v.stack[spStart] = &Array{Value: args}
						sp = spStart + 1
					}
				}
				if numArgs != callee.NumParameters {
					if callee.VarArgs {
						v.err = fmt.Errorf(
							"wrong number of arguments: want>=%d, got=%d",
							callee.NumParameters-1, numArgs)
					} else {
						v.err = fmt.Errorf(
							"wrong number of arguments: want=%d, got=%d",
							callee.NumParameters, numArgs)
					}
					v.ip = ip
					v.sp = sp
					return
				}

				// test if it's tail-call
				if callee == v.curFrame.fn { // recursion
					nextOp := curInsts[ip+1]
					if nextOp == parser.OpReturn ||
						(nextOp == parser.OpPop &&
							parser.OpReturn == curInsts[ip+2]) {
						for p := 0; p < numArgs; p++ {
							v.stack[bp+p] =
								v.stack[sp-numArgs+p]
						}
						sp -= numArgs + 1
						ip = -1 // reset IP to beginning of the frame
						continue
					}
				}
				if v.framesIndex >= MaxFrames {
					v.err = ErrStackOverflow
					v.ip = ip
					v.sp = sp
					return
				}

				if atomic.LoadInt64(&v.stopping) != 0 {
					v.ip = ip
					v.sp = sp
					return
				}
				// update call frame — flush ip/sp into current frame first
				v.curFrame.ip = ip
				v.curFrame = &(v.frames[v.framesIndex])
				v.curFrame.fn = callee
				v.curFrame.freeVars = callee.Free
				v.curFrame.basePointer = sp - numArgs
				bp = sp - numArgs
				v.framesIndex++
				sp = sp - numArgs + callee.NumLocals
				// reload locals from new frame
				ip = -1
				curInsts = callee.Instructions
				if hookMask&HookMaskCall != 0 {
					v.hookFunc(v, HookInfo{
						Event: HookCall,
						Depth: v.framesIndex,
						Pos:   v.fileSet.Position(callee.SourcePos(0)),
					})
					hookMask = v.hookMask
				}
			} else if callee, ok := value.(*InteropFunction); ok {
				var args []Object
				args = append(args, v.stack[sp-numArgs:sp]...)
				// Sync hoisted locals back so RunCompiledFunction sees the
				// correct stack pointer and instruction position if the
				// InteropFunction calls back into the VM.
				v.sp = sp
				v.ip = ip
				ret, e := callee.Value(v, args...)
				sp -= numArgs + 1
				if e != nil {
					if e == ErrWrongNumArguments {
						v.err = fmt.Errorf(
							"wrong number of arguments in call to '%s'",
							value.TypeName())
						v.ip = ip
						v.sp = sp
						return
					}
					if e, ok := e.(ErrInvalidArgumentType); ok {
						v.err = fmt.Errorf(
							"invalid type for argument '%s' in call to '%s': "+
								"expected %s, found %s",
							e.Name, value.TypeName(), e.Expected, e.Found)
						v.ip = ip
						v.sp = sp
						return
					}
					v.err = e
					v.ip = ip
					v.sp = sp
					return
				}
				if ret == nil {
					ret = UndefinedValue
				}
				v.allocs--
				if v.allocs == 0 {
					v.err = ErrObjectAllocLimit
					v.ip = ip
					v.sp = sp
					return
				}
				v.stack[sp] = ret
				sp++
				if atomic.LoadInt64(&v.stopping) != 0 {
					v.ip = ip
					v.sp = sp
					return
				}
			} else {
				if !value.CanCall() {
					v.err = fmt.Errorf("not callable: %s", value.TypeName())
					v.ip = ip
					v.sp = sp
					return
				}
				var args []Object
				args = append(args, v.stack[sp-numArgs:sp]...)
				ret, e := value.Call(args...)
				sp -= numArgs + 1

				// runtime error
				if e != nil {
					if e == ErrWrongNumArguments {
						v.err = fmt.Errorf(
							"wrong number of arguments in call to '%s'",
							value.TypeName())
						v.ip = ip
						v.sp = sp
						return
					}
					if e, ok := e.(ErrInvalidArgumentType); ok {
						v.err = fmt.Errorf(
							"invalid type for argument '%s' in call to '%s': "+
								"expected %s, found %s",
							e.Name, value.TypeName(), e.Expected, e.Found)
						v.ip = ip
						v.sp = sp
						return
					}
					v.err = e
					v.ip = ip
					v.sp = sp
					return
				}

				// nil return -> undefined
				if ret == nil {
					ret = UndefinedValue
				}
				v.allocs--
				if v.allocs == 0 {
					v.err = ErrObjectAllocLimit
					v.ip = ip
					v.sp = sp
					return
				}
				v.stack[sp] = ret
				sp++
				if atomic.LoadInt64(&v.stopping) != 0 {
					v.ip = ip
					v.sp = sp
					return
				}
			}
		case parser.OpReturn:
			ip++
			n := int(curInsts[ip])
			var retVal Object
			switch n {
			case 0:
				retVal = UndefinedValue
			case 1:
				retVal = v.stack[sp-1]
			default:
				vals := make([]Object, n)
				for i := 0; i < n; i++ {
					vals[i] = v.stack[sp-n+i]
				}
				retVal = &MultiValue{Values: vals}
			}
			if hookMask&HookMaskReturn != 0 {
				v.hookFunc(v, HookInfo{
					Event:  HookReturn,
					Depth:  v.framesIndex,
					Pos:    v.fileSet.Position(v.curFrame.fn.SourcePos(ip)),
					RetVal: retVal,
				})
				hookMask = v.hookMask
			}
			v.framesIndex--
			v.curFrame = &v.frames[v.framesIndex-1]
			// reload locals from the returning frame
			curInsts = v.curFrame.fn.Instructions
			ip = v.curFrame.ip
			bp = v.curFrame.basePointer
			sp = v.frames[v.framesIndex].basePointer
			// skip stack overflow check because (newSP) <= (oldSP)
			v.stack[sp-1] = retVal
			if v.resumeDepth != 0 && v.framesIndex == v.resumeDepth {
				v.ip = ip
				v.sp = sp
				return
			}
			//v.sp++
		case parser.OpUnpack:
			ip++
			n := int(curInsts[ip])
			val := v.stack[sp-1]
			sp--
			if mv, ok := val.(*MultiValue); ok {
				for i := 0; i < n; i++ {
					if i < len(mv.Values) {
						v.stack[sp] = mv.Values[i]
					} else {
						v.stack[sp] = UndefinedValue
					}
					sp++
				}
			} else {
				v.stack[sp] = val
				sp++
				for i := 1; i < n; i++ {
					v.stack[sp] = UndefinedValue
					sp++
				}
			}
		case parser.OpDefineLocal:
			ip++
			localIndex := int(curInsts[ip])
			lsp := bp + localIndex

			// local variables can be mutated by other actions
			// so always store the copy of popped value
			val := unwrapMultiValue(v.stack[sp-1])
			sp--
			// Snapshot *StringBuilder to *String to preserve value semantics:
			// b := a  must give b the current value of a, not a live reference.
			if sb, ok := val.(*StringBuilder); ok {
				val = sb.materialize()
			}
			v.stack[lsp] = val
		case parser.OpSetLocal:
			localIndex := int(curInsts[ip+1])
			ip++
			lsp := bp + localIndex

			// update pointee of v.stack[lsp] instead of replacing the pointer
			// itself. this is needed because there can be free variables
			// referencing the same local variables.
			val := unwrapMultiValue(v.stack[sp-1])
			sp--
			if obj, ok := v.stack[lsp].(*ObjectPtr); ok {
				*obj.Value = val
				val = obj
			}
			v.stack[lsp] = val // also use a copy of popped value
		case parser.OpSetSelLocal:
			localIndex := int(curInsts[ip+1])
			numSelectors := int(curInsts[ip+2])
			ip += 2

			// selectors and RHS value
			selectors := make([]Object, numSelectors)
			for i := 0; i < numSelectors; i++ {
				selectors[i] = v.stack[sp-numSelectors+i]
			}
			val := v.stack[sp-numSelectors-1]
			sp -= numSelectors + 1
			dst := v.stack[bp+localIndex]
			if obj, ok := dst.(*ObjectPtr); ok {
				dst = *obj.Value
			}
			if e := indexAssign(dst, val, selectors); e != nil {
				v.err = e
				v.ip = ip
				v.sp = sp
				return
			}
		case parser.OpGetLocal:
			ip++
			localIndex := int(curInsts[ip])
			val := v.stack[bp+localIndex]
			if obj, ok := val.(*ObjectPtr); ok {
				val = *obj.Value
			}
			v.stack[sp] = val
			sp++
		case parser.OpGetBuiltin:
			ip++
			builtinIndex := int(curInsts[ip])
			v.stack[sp] = builtinFuncs[builtinIndex]
			sp++
		case parser.OpClosure:
			ip += 3
			constIndex := int(curInsts[ip-1]) | int(curInsts[ip-2])<<8
			numFree := int(curInsts[ip])
			fn, ok := v.constants[constIndex].(*CompiledFunction)
			if !ok {
				v.err = fmt.Errorf("not function: %s", fn.TypeName())
				v.ip = ip
				v.sp = sp
				return
			}
			free := make([]*ObjectPtr, numFree)
			for i := 0; i < numFree; i++ {
				switch freeVar := (v.stack[sp-numFree+i]).(type) {
				case *ObjectPtr:
					free[i] = freeVar
				default:
					free[i] = &ObjectPtr{
						Value: &v.stack[sp-numFree+i],
					}
				}
			}
			sp -= numFree
			cl := &CompiledFunction{
				Instructions:  fn.Instructions,
				NumLocals:     fn.NumLocals,
				NumParameters: fn.NumParameters,
				VarArgs:       fn.VarArgs,
				SourceMap:     fn.SourceMap,
				Free:          free,
			}
			v.allocs--
			if v.allocs == 0 {
				v.err = ErrObjectAllocLimit
				v.ip = ip
				v.sp = sp
				return
			}
			v.stack[sp] = cl
			sp++
		case parser.OpGetFreePtr:
			ip++
			freeIndex := int(curInsts[ip])
			val := v.curFrame.freeVars[freeIndex]
			v.stack[sp] = val
			sp++
		case parser.OpGetFree:
			ip++
			freeIndex := int(curInsts[ip])
			val := *v.curFrame.freeVars[freeIndex].Value
			v.stack[sp] = val
			sp++
		case parser.OpSetFree:
			ip++
			freeIndex := int(curInsts[ip])
			*v.curFrame.freeVars[freeIndex].Value = unwrapMultiValue(v.stack[sp-1])
			sp--
		case parser.OpGetLocalPtr:
			ip++
			localIndex := int(curInsts[ip])
			lsp := bp + localIndex
			val := v.stack[lsp]
			var freeVar *ObjectPtr
			if obj, ok := val.(*ObjectPtr); ok {
				freeVar = obj
			} else {
				freeVar = &ObjectPtr{Value: &val}
				v.stack[lsp] = freeVar
			}
			v.stack[sp] = freeVar
			sp++
		case parser.OpSetSelFree:
			ip += 2
			freeIndex := int(curInsts[ip-1])
			numSelectors := int(curInsts[ip])

			// selectors and RHS value
			selectors := make([]Object, numSelectors)
			for i := 0; i < numSelectors; i++ {
				selectors[i] = v.stack[sp-numSelectors+i]
			}
			val := v.stack[sp-numSelectors-1]
			sp -= numSelectors + 1
			e := indexAssign(*v.curFrame.freeVars[freeIndex].Value,
				val, selectors)
			if e != nil {
				v.err = e
				v.ip = ip
				v.sp = sp
				return
			}
		case parser.OpIteratorInit:
			var iterator Object
			dst := v.stack[sp-1]
			sp--
			if !dst.CanIterate() {
				v.err = fmt.Errorf("not iterable: %s", dst.TypeName())
				v.ip = ip
				v.sp = sp
				return
			}
			iterator = dst.Iterate()
			v.allocs--
			if v.allocs == 0 {
				v.err = ErrObjectAllocLimit
				v.ip = ip
				v.sp = sp
				return
			}
			v.stack[sp] = iterator
			sp++
		case parser.OpIteratorNext:
			iterator := v.stack[sp-1]
			sp--
			hasMore := iterator.(Iterator).Next()
			if hasMore {
				v.stack[sp] = TrueValue
			} else {
				v.stack[sp] = FalseValue
			}
			sp++
		case parser.OpIteratorKey:
			iterator := v.stack[sp-1]
			sp--
			val := iterator.(Iterator).Key()
			v.stack[sp] = val
			sp++
		case parser.OpIteratorValue:
			iterator := v.stack[sp-1]
			sp--
			val := iterator.(Iterator).Value()
			v.stack[sp] = val
			sp++
		case parser.OpSuspend:
			v.ip = ip
			v.sp = sp
			v.curInsts = curInsts
			return
		case parser.OpDup:
			v.stack[sp] = v.stack[sp-1]
			sp++
		case parser.OpSwap:
			v.stack[sp-1], v.stack[sp-2] = v.stack[sp-2], v.stack[sp-1]
		default:
			v.err = fmt.Errorf("unknown opcode: %d", curInsts[ip])
			v.ip = ip
			v.sp = sp
			return
		}
	}
}

// RunCompiledFunction calls a CompiledFunction from within an InteropFunc,
// re-entering the running VM. It must only be called during an active VM
// execution (i.e. from inside an InteropFunc invoked by the VM).
//
// It is also used by VM.Call for host-side calls into a fresh VM.
func (v *VM) RunCompiledFunction(fn *CompiledFunction, args ...Object) (Object, error) {
	numArgs := len(args)

	if fn.VarArgs {
		realArgs := fn.NumParameters - 1
		varArgs := numArgs - realArgs

		if varArgs < 0 {
			return nil, fmt.Errorf(
				"wrong number of arguments: want>=%d, got=%d",
				realArgs,
				numArgs,
			)
		}

		packed := make([]Object, varArgs)
		copy(packed, args[realArgs:])

		newArgs := make([]Object, realArgs+1)
		copy(newArgs, args[:realArgs])
		newArgs[realArgs] = &Array{Value: packed}

		args = newArgs
		numArgs = len(args)
	}

	if numArgs != fn.NumParameters {
		return nil, fmt.Errorf(
			"wrong number of arguments: want=%d, got=%d",
			fn.NumParameters,
			numArgs,
		)
	}

	if v.framesIndex >= MaxFrames {
		return nil, ErrStackOverflow
	}

	savedSP := v.sp
	savedIP := v.ip
	savedCurFrame := v.curFrame
	savedCurInsts := v.curInsts
	savedFramesIndex := v.framesIndex

	// Push a placeholder that OpReturn will overwrite with the return value
	// (mirrors the callee slot in normal OpCall).
	v.stack[v.sp] = UndefinedValue
	v.sp++

	for _, arg := range args {
		v.stack[v.sp] = arg
		v.sp++
	}

	// Save the current frame's ip so OpReturn can restore it correctly.
	v.curFrame.ip = v.ip

	// Tell run() to stop when this function returns to the current depth.
	prevDepth := v.resumeDepth
	v.resumeDepth = v.framesIndex

	v.curFrame = &v.frames[v.framesIndex]
	v.curFrame.fn = fn
	v.curFrame.freeVars = fn.Free
	v.curFrame.ip = -1
	v.curFrame.basePointer = v.sp - numArgs

	v.curInsts = fn.Instructions
	v.ip = -1
	v.framesIndex++
	v.sp = v.curFrame.basePointer + fn.NumLocals

	atomic.StoreInt64(&v.stopping, 0)

	v.run()

	v.resumeDepth = prevDepth

	if v.err != nil {
		err := v.err
		v.err = nil

		v.sp = savedSP
		v.ip = savedIP
		v.curFrame = savedCurFrame
		v.curInsts = savedCurInsts
		v.framesIndex = savedFramesIndex

		return nil, err
	}

	// OpReturn placed the return value at sp-1; pop the placeholder slot.
	ret := v.stack[v.sp-1]
	v.sp--

	if ret == nil {
		ret = UndefinedValue
	}

	return ret, nil
}

// Call invokes a callable object directly without executing the bytecode main
// function.
//
// This is intended for host-side calls into already-initialized compiled
// scripts, e.g. Compiled.Call("_process", delta).
func (v *VM) Call(fn Object, args ...Object) (Object, error) {
	if fn == nil || fn == UndefinedValue {
		return UndefinedValue, fmt.Errorf("not callable: undefined")
	}

	switch callee := fn.(type) {
	case *CompiledFunction:
		return v.RunCompiledFunction(callee, args...)

	case *InteropFunction:
		ret, err := callee.Value(v, args...)
		if err != nil {
			return UndefinedValue, normalizeCallError(fn, err)
		}
		if ret == nil {
			return UndefinedValue, nil
		}
		return ret, nil

	default:
		if !fn.CanCall() {
			return UndefinedValue, fmt.Errorf("not callable: %s", fn.TypeName())
		}

		ret, err := fn.Call(args...)
		if err != nil {
			return UndefinedValue, normalizeCallError(fn, err)
		}
		if ret == nil {
			return UndefinedValue, nil
		}
		return ret, nil
	}
}

func normalizeCallError(fn Object, err error) error {
	if err == nil {
		return nil
	}

	if err == ErrWrongNumArguments {
		return fmt.Errorf("wrong number of arguments in call to '%s'", fn.TypeName())
	}

	if e, ok := err.(ErrInvalidArgumentType); ok {
		return fmt.Errorf(
			"invalid type for argument '%s' in call to '%s': expected %s, found %s",
			e.Name,
			fn.TypeName(),
			e.Expected,
			e.Found,
		)
	}

	return err
}

// IsStackEmpty tests if the stack is empty or not.
func (v *VM) IsStackEmpty() bool {
	return v.sp == 0
}

// Constants returns the constants pool shared across all VMs running the same bytecode.
func (v *VM) Constants() []Object {
	return v.constants
}

// SourceFileSet returns the source file set used for position reporting.
func (v *VM) SourceFileSet() *parser.SourceFileSet {
	return v.fileSet
}

// VMGlobals returns the global variable slice. The slice is shared with
// child VMs (e.g. coroutines) so writes are visible to all VMs using it.
func (v *VM) VMGlobals() []Object {
	return v.globals
}

func indexAssign(dst, src Object, selectors []Object) error {
	numSel := len(selectors)
	for sidx := numSel - 1; sidx > 0; sidx-- {
		next, err := dst.IndexGet(selectors[sidx])
		if err != nil {
			if err == ErrNotIndexable {
				return fmt.Errorf("not indexable: %s", dst.TypeName())
			}
			if err == ErrInvalidIndexType {
				return fmt.Errorf("invalid index type: %s",
					selectors[sidx].TypeName())
			}
			return err
		}
		dst = next
	}

	if err := dst.IndexSet(selectors[0], src); err != nil {
		if err == ErrNotIndexAssignable {
			return fmt.Errorf("not index-assignable: %s", dst.TypeName())
		}
		if err == ErrInvalidIndexValueType {
			return fmt.Errorf("invaid index value type: %s", src.TypeName())
		}
		return err
	}
	return nil
}
