package tengo_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ganehag/tengo/v3"
)

func lookupBuiltin(name string) func(...tengo.Object) (tengo.Object, error) {
	for _, f := range tengo.GetAllBuiltinFunctions() {
		if f.Name == name {
			return f.Value
		}
	}
	return nil
}

func Test_builtinDelete(t *testing.T) {
	var builtinDelete func(args ...tengo.Object) (tengo.Object, error)
	for _, f := range tengo.GetAllBuiltinFunctions() {
		if f.Name == "delete" {
			builtinDelete = f.Value
			break
		}
	}
	if builtinDelete == nil {
		t.Fatal("builtin delete not found")
	}
	type args struct {
		args []tengo.Object
	}
	tests := []struct {
		name      string
		args      args
		want      tengo.Object
		wantErr   bool
		wantedErr error
		target    interface{}
	}{
		{name: "invalid-arg", args: args{[]tengo.Object{&tengo.String{},
			&tengo.String{}}}, wantErr: true,
			wantedErr: tengo.ErrInvalidArgumentType{
				Name:     "first",
				Expected: "map",
				Found:    "string"},
		},
		{name: "no-args",
			wantErr: true, wantedErr: tengo.ErrWrongNumArguments},
		{name: "empty-args", args: args{[]tengo.Object{}}, wantErr: true,
			wantedErr: tengo.ErrWrongNumArguments,
		},
		{name: "3-args", args: args{[]tengo.Object{
			(*tengo.Map)(nil), (*tengo.String)(nil), (*tengo.String)(nil)}},
			wantErr: true, wantedErr: tengo.ErrWrongNumArguments,
		},
		{name: "nil-map-empty-key",
			args: args{[]tengo.Object{&tengo.Map{}, &tengo.String{}}},
			want: tengo.UndefinedValue,
		},
		{name: "nil-map-nonstr-key",
			args: args{[]tengo.Object{
				&tengo.Map{}, tengo.Int{}}}, wantErr: true,
			wantedErr: tengo.ErrInvalidArgumentType{
				Name: "second", Expected: "string", Found: "int"},
		},
		{name: "nil-map-no-key",
			args: args{[]tengo.Object{&tengo.Map{}}}, wantErr: true,
			wantedErr: tengo.ErrWrongNumArguments,
		},
		{name: "map-missing-key",
			args: args{
				[]tengo.Object{
					&tengo.Map{Value: map[string]tengo.Object{
						"key": &tengo.String{Value: "value"},
					}},
					&tengo.String{Value: "key1"}}},
			want: tengo.UndefinedValue,
			target: &tengo.Map{
				Value: map[string]tengo.Object{
					"key": &tengo.String{
						Value: "value"}}},
		},
		{name: "map-emptied",
			args: args{
				[]tengo.Object{
					&tengo.Map{Value: map[string]tengo.Object{
						"key": &tengo.String{Value: "value"},
					}},
					&tengo.String{Value: "key"}}},
			want:   tengo.UndefinedValue,
			target: &tengo.Map{Value: map[string]tengo.Object{}},
		},
		{name: "map-multi-keys",
			args: args{
				[]tengo.Object{
					&tengo.Map{Value: map[string]tengo.Object{
						"key1": &tengo.String{Value: "value1"},
						"key2": tengo.Int{Value: 10},
					}},
					&tengo.String{Value: "key1"}}},
			want: tengo.UndefinedValue,
			target: &tengo.Map{Value: map[string]tengo.Object{
				"key2": tengo.Int{Value: 10}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := builtinDelete(tt.args.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("builtinDelete() error = %v, wantErr %v",
					err, tt.wantErr)
				return
			}
			if tt.wantErr && !errors.Is(err, tt.wantedErr) {
				if err.Error() != tt.wantedErr.Error() {
					t.Errorf("builtinDelete() error = %v, wantedErr %v",
						err, tt.wantedErr)
					return
				}
			}
			if got != tt.want {
				t.Errorf("builtinDelete() = %v, want %v", got, tt.want)
				return
			}
			if !tt.wantErr && tt.target != nil {
				switch v := tt.args.args[0].(type) {
				case *tengo.Map, *tengo.Array:
					if !reflect.DeepEqual(tt.target, tt.args.args[0]) {
						t.Errorf("builtinDelete() objects are not equal "+
							"got: %+v, want: %+v", tt.args.args[0], tt.target)
					}
				default:
					t.Errorf("builtinDelete() unsuporrted arg[0] type %s",
						v.TypeName())
					return
				}
			}
		})
	}
}

func Test_builtinSplice(t *testing.T) {
	var builtinSplice func(args ...tengo.Object) (tengo.Object, error)
	for _, f := range tengo.GetAllBuiltinFunctions() {
		if f.Name == "splice" {
			builtinSplice = f.Value
			break
		}
	}
	if builtinSplice == nil {
		t.Fatal("builtin splice not found")
	}
	tests := []struct {
		name      string
		args      []tengo.Object
		deleted   tengo.Object
		Array     *tengo.Array
		wantErr   bool
		wantedErr error
	}{
		{name: "no args", args: []tengo.Object{}, wantErr: true,
			wantedErr: tengo.ErrWrongNumArguments,
		},
		{name: "invalid args", args: []tengo.Object{&tengo.Map{}},
			wantErr: true,
			wantedErr: tengo.ErrInvalidArgumentType{
				Name: "first", Expected: "array", Found: "map"},
		},
		{name: "invalid args",
			args:    []tengo.Object{&tengo.Array{}, &tengo.String{}},
			wantErr: true,
			wantedErr: tengo.ErrInvalidArgumentType{
				Name: "second", Expected: "int", Found: "string"},
		},
		{name: "negative index",
			args:      []tengo.Object{&tengo.Array{}, tengo.Int{Value: -1}},
			wantErr:   true,
			wantedErr: tengo.ErrIndexOutOfBounds},
		{name: "non int count",
			args: []tengo.Object{
				&tengo.Array{}, tengo.Int{Value: 0},
				&tengo.String{Value: ""}},
			wantErr: true,
			wantedErr: tengo.ErrInvalidArgumentType{
				Name: "third", Expected: "int", Found: "string"},
		},
		{name: "negative count",
			args: []tengo.Object{
				&tengo.Array{Value: []tengo.Object{
					tengo.Int{Value: 0},
					tengo.Int{Value: 1},
					tengo.Int{Value: 2}}},
				tengo.Int{Value: 0},
				tengo.Int{Value: -1}},
			wantErr:   true,
			wantedErr: tengo.ErrIndexOutOfBounds,
		},
		{name: "insert with zero count",
			args: []tengo.Object{
				&tengo.Array{Value: []tengo.Object{
					tengo.Int{Value: 0},
					tengo.Int{Value: 1},
					tengo.Int{Value: 2}}},
				tengo.Int{Value: 0},
				tengo.Int{Value: 0},
				&tengo.String{Value: "b"}},
			deleted: &tengo.Array{Value: []tengo.Object{}},
			Array: &tengo.Array{Value: []tengo.Object{
				&tengo.String{Value: "b"},
				tengo.Int{Value: 0},
				tengo.Int{Value: 1},
				tengo.Int{Value: 2}}},
		},
		{name: "insert",
			args: []tengo.Object{
				&tengo.Array{Value: []tengo.Object{
					tengo.Int{Value: 0},
					tengo.Int{Value: 1},
					tengo.Int{Value: 2}}},
				tengo.Int{Value: 1},
				tengo.Int{Value: 0},
				&tengo.String{Value: "c"},
				&tengo.String{Value: "d"}},
			deleted: &tengo.Array{Value: []tengo.Object{}},
			Array: &tengo.Array{Value: []tengo.Object{
				tengo.Int{Value: 0},
				&tengo.String{Value: "c"},
				&tengo.String{Value: "d"},
				tengo.Int{Value: 1},
				tengo.Int{Value: 2}}},
		},
		{name: "insert with zero count",
			args: []tengo.Object{
				&tengo.Array{Value: []tengo.Object{
					tengo.Int{Value: 0},
					tengo.Int{Value: 1},
					tengo.Int{Value: 2}}},
				tengo.Int{Value: 1},
				tengo.Int{Value: 0},
				&tengo.String{Value: "c"},
				&tengo.String{Value: "d"}},
			deleted: &tengo.Array{Value: []tengo.Object{}},
			Array: &tengo.Array{Value: []tengo.Object{
				tengo.Int{Value: 0},
				&tengo.String{Value: "c"},
				&tengo.String{Value: "d"},
				tengo.Int{Value: 1},
				tengo.Int{Value: 2}}},
		},
		{name: "insert with delete",
			args: []tengo.Object{
				&tengo.Array{Value: []tengo.Object{
					tengo.Int{Value: 0},
					tengo.Int{Value: 1},
					tengo.Int{Value: 2}}},
				tengo.Int{Value: 1},
				tengo.Int{Value: 1},
				&tengo.String{Value: "c"},
				&tengo.String{Value: "d"}},
			deleted: &tengo.Array{
				Value: []tengo.Object{tengo.Int{Value: 1}}},
			Array: &tengo.Array{Value: []tengo.Object{
				tengo.Int{Value: 0},
				&tengo.String{Value: "c"},
				&tengo.String{Value: "d"},
				tengo.Int{Value: 2}}},
		},
		{name: "insert with delete multi",
			args: []tengo.Object{
				&tengo.Array{Value: []tengo.Object{
					tengo.Int{Value: 0},
					tengo.Int{Value: 1},
					tengo.Int{Value: 2}}},
				tengo.Int{Value: 1},
				tengo.Int{Value: 2},
				&tengo.String{Value: "c"},
				&tengo.String{Value: "d"}},
			deleted: &tengo.Array{Value: []tengo.Object{
				tengo.Int{Value: 1},
				tengo.Int{Value: 2}}},
			Array: &tengo.Array{
				Value: []tengo.Object{
					tengo.Int{Value: 0},
					&tengo.String{Value: "c"},
					&tengo.String{Value: "d"}}},
		},
		{name: "delete all with positive count",
			args: []tengo.Object{
				&tengo.Array{Value: []tengo.Object{
					tengo.Int{Value: 0},
					tengo.Int{Value: 1},
					tengo.Int{Value: 2}}},
				tengo.Int{Value: 0},
				tengo.Int{Value: 3}},
			deleted: &tengo.Array{Value: []tengo.Object{
				tengo.Int{Value: 0},
				tengo.Int{Value: 1},
				tengo.Int{Value: 2}}},
			Array: &tengo.Array{Value: []tengo.Object{}},
		},
		{name: "delete all with big count",
			args: []tengo.Object{
				&tengo.Array{Value: []tengo.Object{
					tengo.Int{Value: 0},
					tengo.Int{Value: 1},
					tengo.Int{Value: 2}}},
				tengo.Int{Value: 0},
				tengo.Int{Value: 5}},
			deleted: &tengo.Array{Value: []tengo.Object{
				tengo.Int{Value: 0},
				tengo.Int{Value: 1},
				tengo.Int{Value: 2}}},
			Array: &tengo.Array{Value: []tengo.Object{}},
		},
		{name: "nothing2",
			args: []tengo.Object{
				&tengo.Array{Value: []tengo.Object{
					tengo.Int{Value: 0},
					tengo.Int{Value: 1},
					tengo.Int{Value: 2}}}},
			Array: &tengo.Array{Value: []tengo.Object{}},
			deleted: &tengo.Array{Value: []tengo.Object{
				tengo.Int{Value: 0},
				tengo.Int{Value: 1},
				tengo.Int{Value: 2}}},
		},
		{name: "pop without count",
			args: []tengo.Object{
				&tengo.Array{Value: []tengo.Object{
					tengo.Int{Value: 0},
					tengo.Int{Value: 1},
					tengo.Int{Value: 2}}},
				tengo.Int{Value: 2}},
			deleted: &tengo.Array{Value: []tengo.Object{tengo.Int{Value: 2}}},
			Array: &tengo.Array{Value: []tengo.Object{
				tengo.Int{Value: 0}, tengo.Int{Value: 1}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := builtinSplice(tt.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("builtinSplice() error = %v, wantErr %v",
					err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.deleted) {
				t.Errorf("builtinSplice() = %v, want %v", got, tt.deleted)
			}
			if tt.wantErr && tt.wantedErr.Error() != err.Error() {
				t.Errorf("builtinSplice() error = %v, wantedErr %v",
					err, tt.wantedErr)
			}
			if tt.Array != nil && !reflect.DeepEqual(tt.Array, tt.args[0]) {
				t.Errorf("builtinSplice() arrays are not equal expected"+
					" %s, got %s", tt.Array, tt.args[0].(*tengo.Array))
			}
		})
	}
}

func Test_builtinRange(t *testing.T) {
	var builtinRange func(args ...tengo.Object) (tengo.Object, error)
	for _, f := range tengo.GetAllBuiltinFunctions() {
		if f.Name == "range" {
			builtinRange = f.Value
			break
		}
	}
	if builtinRange == nil {
		t.Fatal("builtin range not found")
	}
	tests := []struct {
		name      string
		args      []tengo.Object
		result    *tengo.Array
		wantErr   bool
		wantedErr error
	}{
		{name: "no args", args: []tengo.Object{}, wantErr: true,
			wantedErr: tengo.ErrWrongNumArguments,
		},
		{name: "single args", args: []tengo.Object{&tengo.Map{}},
			wantErr:   true,
			wantedErr: tengo.ErrWrongNumArguments,
		},
		{name: "4 args", args: []tengo.Object{&tengo.Map{}, &tengo.String{}, &tengo.String{}, &tengo.String{}},
			wantErr:   true,
			wantedErr: tengo.ErrWrongNumArguments,
		},
		{name: "invalid start",
			args:    []tengo.Object{&tengo.String{}, &tengo.String{}},
			wantErr: true,
			wantedErr: tengo.ErrInvalidArgumentType{
				Name: "start", Expected: "int", Found: "string"},
		},
		{name: "invalid stop",
			args:    []tengo.Object{tengo.Int{}, &tengo.String{}},
			wantErr: true,
			wantedErr: tengo.ErrInvalidArgumentType{
				Name: "stop", Expected: "int", Found: "string"},
		},
		{name: "invalid step",
			args:    []tengo.Object{tengo.Int{}, tengo.Int{}, &tengo.String{}},
			wantErr: true,
			wantedErr: tengo.ErrInvalidArgumentType{
				Name: "step", Expected: "int", Found: "string"},
		},
		{name: "zero step",
			args:      []tengo.Object{tengo.Int{}, tengo.Int{}, tengo.Int{}}, //must greate than 0
			wantErr:   true,
			wantedErr: tengo.ErrInvalidRangeStep,
		},
		{name: "negative step",
			args:      []tengo.Object{tengo.Int{}, tengo.Int{}, intObject(-2)}, //must greate than 0
			wantErr:   true,
			wantedErr: tengo.ErrInvalidRangeStep,
		},
		{name: "same bound",
			args:    []tengo.Object{tengo.Int{}, tengo.Int{}},
			wantErr: false,
			result: &tengo.Array{
				Value: nil,
			},
		},
		{name: "positive range",
			args:    []tengo.Object{tengo.Int{}, tengo.Int{Value: 5}},
			wantErr: false,
			result: &tengo.Array{
				Value: []tengo.Object{
					intObject(0),
					intObject(1),
					intObject(2),
					intObject(3),
					intObject(4),
				},
			},
		},
		{name: "negative range",
			args:    []tengo.Object{tengo.Int{}, tengo.Int{Value: -5}},
			wantErr: false,
			result: &tengo.Array{
				Value: []tengo.Object{
					intObject(0),
					intObject(-1),
					intObject(-2),
					intObject(-3),
					intObject(-4),
				},
			},
		},

		{name: "positive with step",
			args:    []tengo.Object{tengo.Int{}, tengo.Int{Value: 5}, tengo.Int{Value: 2}},
			wantErr: false,
			result: &tengo.Array{
				Value: []tengo.Object{
					intObject(0),
					intObject(2),
					intObject(4),
				},
			},
		},

		{name: "negative with step",
			args:    []tengo.Object{tengo.Int{}, tengo.Int{Value: -10}, tengo.Int{Value: 2}},
			wantErr: false,
			result: &tengo.Array{
				Value: []tengo.Object{
					intObject(0),
					intObject(-2),
					intObject(-4),
					intObject(-6),
					intObject(-8),
				},
			},
		},

		{name: "large range",
			args:    []tengo.Object{intObject(-10), intObject(10), tengo.Int{Value: 3}},
			wantErr: false,
			result: &tengo.Array{
				Value: []tengo.Object{
					intObject(-10),
					intObject(-7),
					intObject(-4),
					intObject(-1),
					intObject(2),
					intObject(5),
					intObject(8),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := builtinRange(tt.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("builtinRange() error = %v, wantErr %v",
					err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.wantedErr.Error() != err.Error() {
				t.Errorf("builtinRange() error = %v, wantedErr %v",
					err, tt.wantedErr)
			}
			if tt.result != nil && !reflect.DeepEqual(tt.result, got) {
				t.Errorf("builtinRange() arrays are not equal expected"+
					" %s, got %s", tt.result, got.(*tengo.Array))
			}
		})
	}
}

func TestBuiltinAppendImmutable(t *testing.T) {
	fn := lookupBuiltin("append")
	src := &tengo.ImmutableArray{Value: []tengo.Object{tengo.Int{Value: 1}, tengo.Int{Value: 2}}}
	got, err := fn(src, tengo.Int{Value: 3})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := got.(*tengo.ImmutableArray)
	if !ok {
		t.Fatalf("expected ImmutableArray, got %T", got)
	}
	if len(result.Value) != 3 || result.Value[2] != (tengo.Int{Value: 3}) {
		t.Fatalf("unexpected result: %v", result)
	}
	// original must be unchanged
	if len(src.Value) != 2 {
		t.Fatal("original was modified")
	}
}

func TestBuiltinAssoc(t *testing.T) {
	assoc := lookupBuiltin("assoc")

	t.Run("mutable array", func(t *testing.T) {
		arr := &tengo.Array{Value: []tengo.Object{tengo.Int{Value: 1}, tengo.Int{Value: 2}}}
		got, err := assoc(arr, tengo.Int{Value: 0}, tengo.Int{Value: 9})
		if err != nil {
			t.Fatal(err)
		}
		result := got.(*tengo.Array)
		if result.Value[0] != (tengo.Int{Value: 9}) {
			t.Fatalf("unexpected: %v", result)
		}
		if arr.Value[0] != (tengo.Int{Value: 1}) {
			t.Fatal("original was modified")
		}
	})

	t.Run("immutable array", func(t *testing.T) {
		arr := &tengo.ImmutableArray{Value: []tengo.Object{tengo.Int{Value: 1}, tengo.Int{Value: 2}}}
		got, err := assoc(arr, tengo.Int{Value: 1}, tengo.Int{Value: 9})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := got.(*tengo.ImmutableArray); !ok {
			t.Fatalf("expected ImmutableArray, got %T", got)
		}
		if got.(*tengo.ImmutableArray).Value[1] != (tengo.Int{Value: 9}) {
			t.Fatalf("unexpected: %v", got)
		}
		if arr.Value[1] != (tengo.Int{Value: 2}) {
			t.Fatal("original was modified")
		}
	})

	t.Run("mutable map", func(t *testing.T) {
		m := &tengo.Map{Value: map[string]tengo.Object{"a": tengo.Int{Value: 1}}}
		got, err := assoc(m, &tengo.String{Value: "b"}, tengo.Int{Value: 2})
		if err != nil {
			t.Fatal(err)
		}
		result := got.(*tengo.Map)
		if result.Value["b"] != (tengo.Int{Value: 2}) {
			t.Fatalf("unexpected: %v", result)
		}
		if _, hasB := m.Value["b"]; hasB {
			t.Fatal("original was modified")
		}
	})

	t.Run("immutable map", func(t *testing.T) {
		m := &tengo.ImmutableMap{Value: map[string]tengo.Object{"a": tengo.Int{Value: 1}}}
		got, err := assoc(m, &tengo.String{Value: "a"}, tengo.Int{Value: 99})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := got.(*tengo.ImmutableMap); !ok {
			t.Fatalf("expected ImmutableMap, got %T", got)
		}
		if got.(*tengo.ImmutableMap).Value["a"] != (tengo.Int{Value: 99}) {
			t.Fatalf("unexpected: %v", got)
		}
		if m.Value["a"] != (tengo.Int{Value: 1}) {
			t.Fatal("original was modified")
		}
	})

	t.Run("out of bounds", func(t *testing.T) {
		arr := &tengo.Array{Value: []tengo.Object{tengo.Int{Value: 1}}}
		_, err := assoc(arr, tengo.Int{Value: 5}, tengo.Int{Value: 9})
		if !errors.Is(err, tengo.ErrIndexOutOfBounds) {
			t.Fatalf("expected ErrIndexOutOfBounds, got %v", err)
		}
	})
}

func TestBuiltinDissoc(t *testing.T) {
	dissoc := lookupBuiltin("dissoc")

	t.Run("mutable map", func(t *testing.T) {
		m := &tengo.Map{Value: map[string]tengo.Object{
			"a": tengo.Int{Value: 1}, "b": tengo.Int{Value: 2},
		}}
		got, err := dissoc(m, &tengo.String{Value: "a"})
		if err != nil {
			t.Fatal(err)
		}
		result := got.(*tengo.Map)
		if _, has := result.Value["a"]; has {
			t.Fatal("key still present")
		}
		if _, has := m.Value["a"]; !has {
			t.Fatal("original was modified")
		}
	})

	t.Run("immutable map", func(t *testing.T) {
		m := &tengo.ImmutableMap{Value: map[string]tengo.Object{
			"a": tengo.Int{Value: 1}, "b": tengo.Int{Value: 2},
		}}
		got, err := dissoc(m, &tengo.String{Value: "b"})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := got.(*tengo.ImmutableMap); !ok {
			t.Fatalf("expected ImmutableMap, got %T", got)
		}
		if _, has := got.(*tengo.ImmutableMap).Value["b"]; has {
			t.Fatal("key still present")
		}
		if _, has := m.Value["b"]; !has {
			t.Fatal("original was modified")
		}
	})

	t.Run("missing key is no-op", func(t *testing.T) {
		m := &tengo.ImmutableMap{Value: map[string]tengo.Object{"a": tengo.Int{Value: 1}}}
		got, err := dissoc(m, &tengo.String{Value: "z"})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.(*tengo.ImmutableMap).Value) != 1 {
			t.Fatal("unexpected size")
		}
	})
}

func TestBuiltinInsert(t *testing.T) {
	insert := lookupBuiltin("insert")

	t.Run("mutable array", func(t *testing.T) {
		arr := &tengo.Array{Value: []tengo.Object{tengo.Int{Value: 1}, tengo.Int{Value: 3}}}
		got, err := insert(arr, tengo.Int{Value: 1}, tengo.Int{Value: 2})
		if err != nil {
			t.Fatal(err)
		}
		result := got.(*tengo.Array)
		if len(result.Value) != 3 || result.Value[1] != (tengo.Int{Value: 2}) {
			t.Fatalf("unexpected: %v", result)
		}
		if len(arr.Value) != 2 {
			t.Fatal("original was modified")
		}
	})

	t.Run("immutable array", func(t *testing.T) {
		arr := &tengo.ImmutableArray{Value: []tengo.Object{tengo.Int{Value: 1}, tengo.Int{Value: 3}}}
		got, err := insert(arr, tengo.Int{Value: 0}, tengo.Int{Value: 0})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := got.(*tengo.ImmutableArray); !ok {
			t.Fatalf("expected ImmutableArray, got %T", got)
		}
		result := got.(*tengo.ImmutableArray)
		if len(result.Value) != 3 || result.Value[0] != (tengo.Int{Value: 0}) {
			t.Fatalf("unexpected: %v", result)
		}
	})

	t.Run("append via insert at len", func(t *testing.T) {
		arr := &tengo.ImmutableArray{Value: []tengo.Object{tengo.Int{Value: 1}}}
		got, err := insert(arr, tengo.Int{Value: 1}, tengo.Int{Value: 2})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.(*tengo.ImmutableArray).Value) != 2 {
			t.Fatal("expected length 2")
		}
	})
}

func TestBuiltinRemove(t *testing.T) {
	remove := lookupBuiltin("remove")

	t.Run("mutable array", func(t *testing.T) {
		arr := &tengo.Array{Value: []tengo.Object{
			tengo.Int{Value: 1}, tengo.Int{Value: 2}, tengo.Int{Value: 3},
		}}
		got, err := remove(arr, tengo.Int{Value: 1})
		if err != nil {
			t.Fatal(err)
		}
		result := got.(*tengo.Array)
		if len(result.Value) != 2 || result.Value[1] != (tengo.Int{Value: 3}) {
			t.Fatalf("unexpected: %v", result)
		}
		if len(arr.Value) != 3 {
			t.Fatal("original was modified")
		}
	})

	t.Run("immutable array", func(t *testing.T) {
		arr := &tengo.ImmutableArray{Value: []tengo.Object{
			tengo.Int{Value: 1}, tengo.Int{Value: 2}, tengo.Int{Value: 3},
		}}
		got, err := remove(arr, tengo.Int{Value: 0})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := got.(*tengo.ImmutableArray); !ok {
			t.Fatalf("expected ImmutableArray, got %T", got)
		}
		result := got.(*tengo.ImmutableArray)
		if len(result.Value) != 2 || result.Value[0] != (tengo.Int{Value: 2}) {
			t.Fatalf("unexpected: %v", result)
		}
	})

	t.Run("out of bounds", func(t *testing.T) {
		arr := &tengo.ImmutableArray{Value: []tengo.Object{tengo.Int{Value: 1}}}
		_, err := remove(arr, tengo.Int{Value: 5})
		if !errors.Is(err, tengo.ErrIndexOutOfBounds) {
			t.Fatalf("expected ErrIndexOutOfBounds, got %v", err)
		}
	})
}
