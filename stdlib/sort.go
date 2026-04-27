package stdlib

import (
	"fmt"
	"sort"

	"github.com/tengolang/tengo/v3"
)

var sortModule = map[string]tengo.Object{
	// ints returns a sorted copy of an int array (ascending).
	"ints": &tengo.UserFunction{
		Name: "ints",
		Value: func(args ...tengo.Object) (tengo.Object, error) {
			if len(args) != 1 {
				return nil, tengo.ErrWrongNumArguments
			}
			items, err := toObjectSlice(args[0], "ints")
			if err != nil {
				return nil, err
			}
			out := make([]tengo.Object, len(items))
			for i, v := range items {
				iv, ok := v.(tengo.Int)
				if !ok {
					return nil, tengo.ErrInvalidArgumentType{
						Name:     fmt.Sprintf("ints[%d]", i),
						Expected: "int",
						Found:    v.TypeName(),
					}
				}
				out[i] = iv
			}
			sort.Slice(out, func(i, j int) bool {
				return out[i].(tengo.Int).Value < out[j].(tengo.Int).Value
			})
			return &tengo.Array{Value: out}, nil
		},
	},

	// floats returns a sorted copy of a float array (ascending).
	"floats": &tengo.UserFunction{
		Name: "floats",
		Value: func(args ...tengo.Object) (tengo.Object, error) {
			if len(args) != 1 {
				return nil, tengo.ErrWrongNumArguments
			}
			items, err := toObjectSlice(args[0], "floats")
			if err != nil {
				return nil, err
			}
			out := make([]tengo.Object, len(items))
			for i, v := range items {
				switch fv := v.(type) {
				case tengo.Float:
					out[i] = fv
				case tengo.Int:
					out[i] = tengo.Float{Value: float64(fv.Value)}
				default:
					return nil, tengo.ErrInvalidArgumentType{
						Name:     fmt.Sprintf("floats[%d]", i),
						Expected: "float or int",
						Found:    v.TypeName(),
					}
				}
			}
			sort.Slice(out, func(i, j int) bool {
				return out[i].(tengo.Float).Value < out[j].(tengo.Float).Value
			})
			return &tengo.Array{Value: out}, nil
		},
	},

	// strings returns a sorted copy of a string array (ascending, lexicographic).
	"strings": &tengo.UserFunction{
		Name: "strings",
		Value: func(args ...tengo.Object) (tengo.Object, error) {
			if len(args) != 1 {
				return nil, tengo.ErrWrongNumArguments
			}
			items, err := toObjectSlice(args[0], "strings")
			if err != nil {
				return nil, err
			}
			out := make([]tengo.Object, len(items))
			for i, v := range items {
				sv, ok := v.(*tengo.String)
				if !ok {
					return nil, tengo.ErrInvalidArgumentType{
						Name:     fmt.Sprintf("strings[%d]", i),
						Expected: "string",
						Found:    v.TypeName(),
					}
				}
				out[i] = sv
			}
			sort.Slice(out, func(i, j int) bool {
				return out[i].(*tengo.String).Value < out[j].(*tengo.String).Value
			})
			return &tengo.Array{Value: out}, nil
		},
	},

	// reverse returns a reversed copy of an array.
	"reverse": &tengo.UserFunction{
		Name: "reverse",
		Value: func(args ...tengo.Object) (tengo.Object, error) {
			if len(args) != 1 {
				return nil, tengo.ErrWrongNumArguments
			}
			items, err := toObjectSlice(args[0], "reverse")
			if err != nil {
				return nil, err
			}
			out := make([]tengo.Object, len(items))
			for i, v := range items {
				out[len(items)-1-i] = v
			}
			return &tengo.Array{Value: out}, nil
		},
	},

	// by returns a sorted copy of an array using a comparator function.
	// fn(a, b) must return true if a should come before b.
	// Accepts both compiled func literals and UserFunctions.
	"by": &tengo.InteropFunction{
		Name: "by",
		Value: func(vm *tengo.VM, args ...tengo.Object) (tengo.Object, error) {
			if len(args) != 2 {
				return nil, tengo.ErrWrongNumArguments
			}
			items, err := toObjectSlice(args[0], "by")
			if err != nil {
				return nil, err
			}
			if !args[1].CanCall() {
				return nil, tengo.ErrInvalidArgumentType{
					Name:     "fn",
					Expected: "func",
					Found:    args[1].TypeName(),
				}
			}

			out := make([]tengo.Object, len(items))
			copy(out, items)

			var sortErr error
			sort.SliceStable(out, func(i, j int) bool {
				if sortErr != nil {
					return false
				}
				var ret tengo.Object
				switch f := args[1].(type) {
				case *tengo.CompiledFunction:
					ret, sortErr = vm.RunCompiledFunction(f, out[i], out[j])
				default:
					ret, sortErr = f.Call(out[i], out[j])
				}
				if sortErr != nil {
					return false
				}
				return !ret.IsFalsy()
			})
			if sortErr != nil {
				return nil, sortErr
			}
			return &tengo.Array{Value: out}, nil
		},
	},
}

// toObjectSlice extracts the element slice from an Array or ImmutableArray.
func toObjectSlice(o tengo.Object, fname string) ([]tengo.Object, error) {
	switch v := o.(type) {
	case *tengo.Array:
		return v.Value, nil
	case *tengo.ImmutableArray:
		return v.Value, nil
	default:
		return nil, tengo.ErrInvalidArgumentType{
			Name:     fname,
			Expected: "array",
			Found:    o.TypeName(),
		}
	}
}
