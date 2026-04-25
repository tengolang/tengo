package stdlib

import (
	"math/rand"
	"sync"

	"github.com/tengolang/tengo/v3"
)

// globalRand is a goroutine-safe source for the module-level rand functions.
// Re-created on seed() to provide deterministic output.
var (
	globalRandMu sync.Mutex
	globalRand   = rand.New(rand.NewSource(rand.Int63())) //nolint:gosec
)

func gRandInt63() int64 {
	globalRandMu.Lock()
	v := globalRand.Int63()
	globalRandMu.Unlock()
	return v
}

func gRandFloat64() float64 {
	globalRandMu.Lock()
	v := globalRand.Float64()
	globalRandMu.Unlock()
	return v
}

func gRandExpFloat64() float64 {
	globalRandMu.Lock()
	v := globalRand.ExpFloat64()
	globalRandMu.Unlock()
	return v
}

func gRandNormFloat64() float64 {
	globalRandMu.Lock()
	v := globalRand.NormFloat64()
	globalRandMu.Unlock()
	return v
}

func gRandPerm(n int) []int {
	globalRandMu.Lock()
	v := globalRand.Perm(n)
	globalRandMu.Unlock()
	return v
}

func gRandInt63n(n int64) int64 {
	globalRandMu.Lock()
	v := globalRand.Int63n(n)
	globalRandMu.Unlock()
	return v
}

func gRandSeed(seed int64) {
	globalRandMu.Lock()
	globalRand = rand.New(rand.NewSource(seed))
	globalRandMu.Unlock()
}

func gRandRead(p []byte) int {
	globalRandMu.Lock()
	readBytes(globalRand, p)
	globalRandMu.Unlock()
	return len(p)
}

// readBytes fills p using r, extracting 7 bytes per Int63() call.
// This replicates the algorithm from (*rand.Rand).Read without using
// the deprecated method.
func readBytes(r *rand.Rand, p []byte) {
	var val int64
	var pos int8
	for i := range p {
		if pos == 0 {
			val = r.Int63()
			pos = 7
		}
		p[i] = byte(val)
		val >>= 8
		pos--
	}
}

var randModule = map[string]tengo.Object{
	"int": &tengo.UserFunction{
		Name:  "int",
		Value: FuncARI64(gRandInt63),
	},
	"float": &tengo.UserFunction{
		Name:  "float",
		Value: FuncARF(gRandFloat64),
	},
	"intn": &tengo.UserFunction{
		Name:  "intn",
		Value: FuncAI64RI64(gRandInt63n),
	},
	"exp_float": &tengo.UserFunction{
		Name:  "exp_float",
		Value: FuncARF(gRandExpFloat64),
	},
	"norm_float": &tengo.UserFunction{
		Name:  "norm_float",
		Value: FuncARF(gRandNormFloat64),
	},
	"perm": &tengo.UserFunction{
		Name:  "perm",
		Value: FuncAIRIs(gRandPerm),
	},
	"seed": &tengo.UserFunction{
		Name:  "seed",
		Value: FuncAI64R(gRandSeed),
	},
	"read": &tengo.UserFunction{
		Name: "read",
		Value: func(args ...tengo.Object) (ret tengo.Object, err error) {
			if len(args) != 1 {
				return nil, tengo.ErrWrongNumArguments
			}
			y1, ok := args[0].(*tengo.Bytes)
			if !ok {
				return nil, tengo.ErrInvalidArgumentType{
					Name:     "first",
					Expected: "bytes",
					Found:    args[0].TypeName(),
				}
			}
			return tengo.Int{Value: int64(gRandRead(y1.Value))}, nil
		},
	},
	"rand": &tengo.UserFunction{
		Name: "rand",
		Value: func(args ...tengo.Object) (tengo.Object, error) {
			if len(args) != 1 {
				return nil, tengo.ErrWrongNumArguments
			}
			i1, ok := tengo.ToInt64(args[0])
			if !ok {
				return nil, tengo.ErrInvalidArgumentType{
					Name:     "first",
					Expected: "int(compatible)",
					Found:    args[0].TypeName(),
				}
			}
			src := rand.NewSource(i1)
			return randRand(rand.New(src)), nil
		},
	},
}

func randRand(r *rand.Rand) *tengo.ImmutableMap {
	// mu protects r since ImmutableMap values may be called concurrently.
	var mu sync.Mutex

	read := func(p []byte) int {
		mu.Lock()
		readBytes(r, p)
		mu.Unlock()
		return len(p)
	}

	return &tengo.ImmutableMap{
		Value: map[string]tengo.Object{
			"int": &tengo.UserFunction{
				Name: "int",
				Value: func(args ...tengo.Object) (tengo.Object, error) {
					if len(args) != 0 {
						return nil, tengo.ErrWrongNumArguments
					}
					mu.Lock()
					v := r.Int63()
					mu.Unlock()
					return tengo.Int{Value: v}, nil
				},
			},
			"float": &tengo.UserFunction{
				Name: "float",
				Value: func(args ...tengo.Object) (tengo.Object, error) {
					if len(args) != 0 {
						return nil, tengo.ErrWrongNumArguments
					}
					mu.Lock()
					v := r.Float64()
					mu.Unlock()
					return tengo.Float{Value: v}, nil
				},
			},
			"intn": &tengo.UserFunction{
				Name: "intn",
				Value: func(args ...tengo.Object) (tengo.Object, error) {
					if len(args) != 1 {
						return nil, tengo.ErrWrongNumArguments
					}
					i1, ok := tengo.ToInt64(args[0])
					if !ok {
						return nil, tengo.ErrInvalidArgumentType{
							Name:     "first",
							Expected: "int(compatible)",
							Found:    args[0].TypeName(),
						}
					}
					mu.Lock()
					v := r.Int63n(i1)
					mu.Unlock()
					return tengo.Int{Value: v}, nil
				},
			},
			"exp_float": &tengo.UserFunction{
				Name: "exp_float",
				Value: func(args ...tengo.Object) (tengo.Object, error) {
					if len(args) != 0 {
						return nil, tengo.ErrWrongNumArguments
					}
					mu.Lock()
					v := r.ExpFloat64()
					mu.Unlock()
					return tengo.Float{Value: v}, nil
				},
			},
			"norm_float": &tengo.UserFunction{
				Name: "norm_float",
				Value: func(args ...tengo.Object) (tengo.Object, error) {
					if len(args) != 0 {
						return nil, tengo.ErrWrongNumArguments
					}
					mu.Lock()
					v := r.NormFloat64()
					mu.Unlock()
					return tengo.Float{Value: v}, nil
				},
			},
			"perm": &tengo.UserFunction{
				Name: "perm",
				Value: func(args ...tengo.Object) (tengo.Object, error) {
					if len(args) != 1 {
						return nil, tengo.ErrWrongNumArguments
					}
					i1, ok := tengo.ToInt(args[0])
					if !ok {
						return nil, tengo.ErrInvalidArgumentType{
							Name:     "first",
							Expected: "int(compatible)",
							Found:    args[0].TypeName(),
						}
					}
					mu.Lock()
					v := r.Perm(i1)
					mu.Unlock()
					arr := make([]tengo.Object, len(v))
					for i, x := range v {
						arr[i] = tengo.Int{Value: int64(x)}
					}
					return &tengo.Array{Value: arr}, nil
				},
			},
			"seed": &tengo.UserFunction{
				Name: "seed",
				Value: func(args ...tengo.Object) (tengo.Object, error) {
					if len(args) != 1 {
						return nil, tengo.ErrWrongNumArguments
					}
					i1, ok := tengo.ToInt64(args[0])
					if !ok {
						return nil, tengo.ErrInvalidArgumentType{
							Name:     "first",
							Expected: "int(compatible)",
							Found:    args[0].TypeName(),
						}
					}
					mu.Lock()
					r = rand.New(rand.NewSource(i1))
					mu.Unlock()
					return tengo.UndefinedValue, nil
				},
			},
			"read": &tengo.UserFunction{
				Name: "read",
				Value: func(args ...tengo.Object) (tengo.Object, error) {
					if len(args) != 1 {
						return nil, tengo.ErrWrongNumArguments
					}
					y1, ok := args[0].(*tengo.Bytes)
					if !ok {
						return nil, tengo.ErrInvalidArgumentType{
							Name:     "first",
							Expected: "bytes",
							Found:    args[0].TypeName(),
						}
					}
					return tengo.Int{Value: int64(read(y1.Value))}, nil
				},
			},
		},
	}
}
