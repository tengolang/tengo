package tengo

import "github.com/ganehag/tengo/v3/parser"

// HookEvent identifies which VM event triggered a hook.
type HookEvent int

const (
	HookCall   HookEvent = iota // a compiled function was called
	HookReturn                  // a function is about to return
	HookLine                    // execution reached a new source line
)

// HookMask is a bitmask that selects which events fire the hook. Combine
// values with bitwise OR: HookMaskCall | HookMaskReturn.
type HookMask int

const (
	HookMaskCall   HookMask = 1 << iota // enable HookCall events
	HookMaskReturn                       // enable HookReturn events
	HookMaskLine                         // enable HookLine events
)

// HookInfo carries the context for a single hook invocation.
type HookInfo struct {
	// Event is the type of event that fired the hook.
	Event HookEvent

	// Depth is the call-stack depth at the time of the event.
	// 1 means the top-level script body; each nested call increments it.
	Depth int

	// Pos is the source position associated with the event:
	//   HookCall   — definition site of the called function
	//   HookReturn — instruction that triggered the return
	//   HookLine   — the new source line being entered
	Pos parser.SourceFilePos

	// RetVal is the value being returned. Set only for HookReturn;
	// nil for all other event types.
	RetVal Object
}

// HookFunc is called by the VM at each traced event. The VM is live during
// the call: the hook may read globals, call Pause(), or inspect Depth/Pos.
// It must not call Run() or Resume().
type HookFunc func(vm *VM, info HookInfo)
