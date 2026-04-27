package stdlib

import (
	"math/rand"
	"sync"

	"github.com/tengolang/tengo/v3"
)

// seededRand is a goroutine-safe rand.Rand wrapper that supports re-seeding
// via seed(), which replaces the internal pointer. Method values on *seededRand
// always call through the pointer field, so they stay correct after re-seeding.
type seededRand struct {
	mu sync.Mutex
	r  *rand.Rand
}

func (s *seededRand) seed(n int64) {
	s.mu.Lock()
	s.r = rand.New(rand.NewSource(n))
	s.mu.Unlock()
}

func (s *seededRand) int63() int64 {
	s.mu.Lock()
	v := s.r.Int63()
	s.mu.Unlock()
	return v
}

func (s *seededRand) float64() float64 {
	s.mu.Lock()
	v := s.r.Float64()
	s.mu.Unlock()
	return v
}

func (s *seededRand) int63n(n int64) int64 {
	s.mu.Lock()
	v := s.r.Int63n(n)
	s.mu.Unlock()
	return v
}

func (s *seededRand) expFloat64() float64 {
	s.mu.Lock()
	v := s.r.ExpFloat64()
	s.mu.Unlock()
	return v
}

func (s *seededRand) normFloat64() float64 {
	s.mu.Lock()
	v := s.r.NormFloat64()
	s.mu.Unlock()
	return v
}

func (s *seededRand) perm(n int) []int {
	s.mu.Lock()
	v := s.r.Perm(n)
	s.mu.Unlock()
	return v
}

func (s *seededRand) read(p []byte) int {
	s.mu.Lock()
	readBytes(s.r, p)
	s.mu.Unlock()
	return len(p)
}

// readBytes fills p using r, extracting 7 bytes per Int63() call.
// This replicates the algorithm from (*rand.Rand).Read without calling
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

// globalRand backs the module-level rand functions.
var globalRand = &seededRand{r: rand.New(rand.NewSource(rand.Int63()))} //nolint:gosec

var randModule = map[string]tengo.Object{
	"int": &tengo.UserFunction{
		Name:  "int",
		Value: FuncARI64(globalRand.int63),
	},
	"float": &tengo.UserFunction{
		Name:  "float",
		Value: FuncARF(globalRand.float64),
	},
	"intn": &tengo.UserFunction{
		Name:  "intn",
		Value: FuncAI64RI64(globalRand.int63n),
	},
	"exp_float": &tengo.UserFunction{
		Name:  "exp_float",
		Value: FuncARF(globalRand.expFloat64),
	},
	"norm_float": &tengo.UserFunction{
		Name:  "norm_float",
		Value: FuncARF(globalRand.normFloat64),
	},
	"perm": &tengo.UserFunction{
		Name:  "perm",
		Value: FuncAIRIs(globalRand.perm),
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
			return tengo.Int{Value: int64(globalRand.read(y1.Value))}, nil
		},
	},
	"new": &tengo.UserFunction{
		Name: "new",
		Value: func(args ...tengo.Object) (tengo.Object, error) {
			if len(args) > 1 {
				return nil, tengo.ErrWrongNumArguments
			}
			var src rand.Source
			if len(args) == 1 {
				i1, ok := tengo.ToInt64(args[0])
				if !ok {
					return nil, tengo.ErrInvalidArgumentType{
						Name:     "first",
						Expected: "int(compatible)",
						Found:    args[0].TypeName(),
					}
				}
				src = rand.NewSource(i1)
			} else {
				src = rand.NewSource(globalRand.int63())
			}
			return randRand(&seededRand{r: rand.New(src)}), nil //nolint:gosec
		},
	},
}

func randRand(sr *seededRand) *tengo.ImmutableMap {
	return &tengo.ImmutableMap{
		Value: map[string]tengo.Object{
			"int": &tengo.UserFunction{
				Name:  "int",
				Value: FuncARI64(sr.int63),
			},
			"float": &tengo.UserFunction{
				Name:  "float",
				Value: FuncARF(sr.float64),
			},
			"intn": &tengo.UserFunction{
				Name:  "intn",
				Value: FuncAI64RI64(sr.int63n),
			},
			"exp_float": &tengo.UserFunction{
				Name:  "exp_float",
				Value: FuncARF(sr.expFloat64),
			},
			"norm_float": &tengo.UserFunction{
				Name:  "norm_float",
				Value: FuncARF(sr.normFloat64),
			},
			"perm": &tengo.UserFunction{
				Name:  "perm",
				Value: FuncAIRIs(sr.perm),
			},
			"seed": &tengo.UserFunction{
				Name:  "seed",
				Value: FuncAI64R(sr.seed),
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
					return tengo.Int{Value: int64(sr.read(y1.Value))}, nil
				},
			},
		},
	}
}
