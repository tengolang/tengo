package tengo

var builtinFuncs = []*BuiltinFunction{
	{
		Name:  "len",
		Value: builtinLen,
	},
	{
		Name:  "copy",
		Value: builtinCopy,
	},
	{
		Name:  "append",
		Value: builtinAppend,
	},
	{
		Name:  "delete",
		Value: builtinDelete,
	},
	{
		Name:  "splice",
		Value: builtinSplice,
	},
	{
		Name:  "string",
		Value: builtinString,
	},
	{
		Name:  "int",
		Value: builtinInt,
	},
	{
		Name:  "bool",
		Value: builtinBool,
	},
	{
		Name:  "float",
		Value: builtinFloat,
	},
	{
		Name:  "char",
		Value: builtinChar,
	},
	{
		Name:  "bytes",
		Value: builtinBytes,
	},
	{
		Name:  "time",
		Value: builtinTime,
	},
	{
		Name:  "is_int",
		Value: builtinIsInt,
	},
	{
		Name:  "is_float",
		Value: builtinIsFloat,
	},
	{
		Name:  "is_string",
		Value: builtinIsString,
	},
	{
		Name:  "is_bool",
		Value: builtinIsBool,
	},
	{
		Name:  "is_char",
		Value: builtinIsChar,
	},
	{
		Name:  "is_bytes",
		Value: builtinIsBytes,
	},
	{
		Name:  "is_array",
		Value: builtinIsArray,
	},
	{
		Name:  "is_immutable_array",
		Value: builtinIsImmutableArray,
	},
	{
		Name:  "is_map",
		Value: builtinIsMap,
	},
	{
		Name:  "is_immutable_map",
		Value: builtinIsImmutableMap,
	},
	{
		Name:  "is_iterable",
		Value: builtinIsIterable,
	},
	{
		Name:  "is_time",
		Value: builtinIsTime,
	},
	{
		Name:  "is_error",
		Value: builtinIsError,
	},
	{
		Name:  "is_undefined",
		Value: builtinIsUndefined,
	},
	{
		Name:  "is_function",
		Value: builtinIsFunction,
	},
	{
		Name:  "is_callable",
		Value: builtinIsCallable,
	},
	{
		Name:  "type_name",
		Value: builtinTypeName,
	},
	{
		Name:  "format",
		Value: builtinFormat,
	},
	{
		Name:  "range",
		Value: builtinRange,
	},
	{
		Name:  "assoc",
		Value: builtinAssoc,
	},
	{
		Name:  "dissoc",
		Value: builtinDissoc,
	},
	{
		Name:  "insert",
		Value: builtinInsert,
	},
	{
		Name:  "remove",
		Value: builtinRemove,
	},
}

// GetAllBuiltinFunctions returns all builtin function objects.
func GetAllBuiltinFunctions() []*BuiltinFunction {
	return append([]*BuiltinFunction{}, builtinFuncs...)
}

func builtinTypeName(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	return &String{Value: args[0].TypeName()}, nil
}

func builtinIsString(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	if _, ok := args[0].(*String); ok {
		return TrueValue, nil
	}
	return FalseValue, nil
}

func builtinIsInt(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	if _, ok := args[0].(Int); ok {
		return TrueValue, nil
	}
	return FalseValue, nil
}

func builtinIsFloat(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	if _, ok := args[0].(Float); ok {
		return TrueValue, nil
	}
	return FalseValue, nil
}

func builtinIsBool(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	if _, ok := args[0].(Bool); ok {
		return TrueValue, nil
	}
	return FalseValue, nil
}

func builtinIsChar(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	if _, ok := args[0].(Char); ok {
		return TrueValue, nil
	}
	return FalseValue, nil
}

func builtinIsBytes(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	if _, ok := args[0].(*Bytes); ok {
		return TrueValue, nil
	}
	return FalseValue, nil
}

func builtinIsArray(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	if _, ok := args[0].(*Array); ok {
		return TrueValue, nil
	}
	return FalseValue, nil
}

func builtinIsImmutableArray(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	if _, ok := args[0].(*ImmutableArray); ok {
		return TrueValue, nil
	}
	return FalseValue, nil
}

func builtinIsMap(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	if _, ok := args[0].(*Map); ok {
		return TrueValue, nil
	}
	return FalseValue, nil
}

func builtinIsImmutableMap(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	if _, ok := args[0].(*ImmutableMap); ok {
		return TrueValue, nil
	}
	return FalseValue, nil
}

func builtinIsTime(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	if _, ok := args[0].(*Time); ok {
		return TrueValue, nil
	}
	return FalseValue, nil
}

func builtinIsError(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	if _, ok := args[0].(*Error); ok {
		return TrueValue, nil
	}
	return FalseValue, nil
}

func builtinIsUndefined(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	if args[0] == UndefinedValue {
		return TrueValue, nil
	}
	return FalseValue, nil
}

func builtinIsFunction(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	switch args[0].(type) {
	case *CompiledFunction:
		return TrueValue, nil
	}
	return FalseValue, nil
}

func builtinIsCallable(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	if args[0].CanCall() {
		return TrueValue, nil
	}
	return FalseValue, nil
}

func builtinIsIterable(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	if args[0].CanIterate() {
		return TrueValue, nil
	}
	return FalseValue, nil
}

// len(obj object) => int
func builtinLen(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	switch arg := args[0].(type) {
	case *Array:
		return Int{Value: int64(len(arg.Value))}, nil
	case *ImmutableArray:
		return Int{Value: int64(len(arg.Value))}, nil
	case *String:
		return Int{Value: int64(len(arg.Value))}, nil
	case *Bytes:
		return Int{Value: int64(len(arg.Value))}, nil
	case *Map:
		return Int{Value: int64(len(arg.Value))}, nil
	case *ImmutableMap:
		return Int{Value: int64(len(arg.Value))}, nil
	default:
		return nil, ErrInvalidArgumentType{
			Name:     "first",
			Expected: "array/string/bytes/map",
			Found:    arg.TypeName(),
		}
	}
}

//range(start, stop[, step])
func builtinRange(args ...Object) (Object, error) {
	numArgs := len(args)
	if numArgs < 2 || numArgs > 3 {
		return nil, ErrWrongNumArguments
	}
	var start, stop Int
	hasStep := false
	var step Int

	for i, arg := range args {
		v, ok := args[i].(Int)
		if !ok {
			var name string
			switch i {
			case 0:
				name = "start"
			case 1:
				name = "stop"
			case 2:
				name = "step"
			}

			return nil, ErrInvalidArgumentType{
				Name:     name,
				Expected: "int",
				Found:    arg.TypeName(),
			}
		}
		if i == 2 && v.Value <= 0 {
			return nil, ErrInvalidRangeStep
		}
		switch i {
		case 0:
			start = v
		case 1:
			stop = v
		case 2:
			step = v
			hasStep = true
		}
	}

	if !hasStep {
		step = Int{Value: int64(1)}
	}

	return buildRange(start.Value, stop.Value, step.Value), nil
}

func buildRange(start, stop, step int64) *Array {
	array := &Array{}
	if start <= stop {
		for i := start; i < stop; i += step {
			array.Value = append(array.Value, Int{
				Value: i,
			})
		}
	} else {
		for i := start; i > stop; i -= step {
			array.Value = append(array.Value, Int{
				Value: i,
			})
		}
	}
	return array
}

func builtinFormat(args ...Object) (Object, error) {
	numArgs := len(args)
	if numArgs == 0 {
		return nil, ErrWrongNumArguments
	}
	format, ok := args[0].(*String)
	if !ok {
		return nil, ErrInvalidArgumentType{
			Name:     "format",
			Expected: "string",
			Found:    args[0].TypeName(),
		}
	}
	if numArgs == 1 {
		// okay to return 'format' directly as String is immutable
		return format, nil
	}
	s, err := Format(format.Value, args[1:]...)
	if err != nil {
		return nil, err
	}
	return &String{Value: s}, nil
}

func builtinCopy(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	return args[0].Copy(), nil
}

func builtinString(args ...Object) (Object, error) {
	argsLen := len(args)
	if !(argsLen == 1 || argsLen == 2) {
		return nil, ErrWrongNumArguments
	}
	if _, ok := args[0].(*String); ok {
		return args[0], nil
	}
	v, ok := ToString(args[0])
	if ok {
		if len(v) > MaxStringLen {
			return nil, ErrStringLimit
		}
		return &String{Value: v}, nil
	}
	if argsLen == 2 {
		return args[1], nil
	}
	return UndefinedValue, nil
}

func builtinInt(args ...Object) (Object, error) {
	argsLen := len(args)
	if !(argsLen == 1 || argsLen == 2) {
		return nil, ErrWrongNumArguments
	}
	if _, ok := args[0].(Int); ok {
		return args[0], nil
	}
	v, ok := ToInt64(args[0])
	if ok {
		return Int{Value: v}, nil
	}
	if argsLen == 2 {
		return args[1], nil
	}
	return UndefinedValue, nil
}

func builtinFloat(args ...Object) (Object, error) {
	argsLen := len(args)
	if !(argsLen == 1 || argsLen == 2) {
		return nil, ErrWrongNumArguments
	}
	if _, ok := args[0].(Float); ok {
		return args[0], nil
	}
	v, ok := ToFloat64(args[0])
	if ok {
		return Float{Value: v}, nil
	}
	if argsLen == 2 {
		return args[1], nil
	}
	return UndefinedValue, nil
}

func builtinBool(args ...Object) (Object, error) {
	if len(args) != 1 {
		return nil, ErrWrongNumArguments
	}
	if _, ok := args[0].(Bool); ok {
		return args[0], nil
	}
	v, ok := ToBool(args[0])
	if ok {
		if v {
			return TrueValue, nil
		}
		return FalseValue, nil
	}
	return UndefinedValue, nil
}

func builtinChar(args ...Object) (Object, error) {
	argsLen := len(args)
	if !(argsLen == 1 || argsLen == 2) {
		return nil, ErrWrongNumArguments
	}
	if _, ok := args[0].(Char); ok {
		return args[0], nil
	}
	v, ok := ToRune(args[0])
	if ok {
		return Char{Value: v}, nil
	}
	if argsLen == 2 {
		return args[1], nil
	}
	return UndefinedValue, nil
}

func builtinBytes(args ...Object) (Object, error) {
	argsLen := len(args)
	if !(argsLen == 1 || argsLen == 2) {
		return nil, ErrWrongNumArguments
	}

	// bytes(N) => create a new bytes with given size N
	if n, ok := args[0].(Int); ok {
		if n.Value > int64(MaxBytesLen) {
			return nil, ErrBytesLimit
		}
		return &Bytes{Value: make([]byte, int(n.Value))}, nil
	}
	v, ok := ToByteSlice(args[0])
	if ok {
		if len(v) > MaxBytesLen {
			return nil, ErrBytesLimit
		}
		return &Bytes{Value: v}, nil
	}
	if argsLen == 2 {
		return args[1], nil
	}
	return UndefinedValue, nil
}

func builtinTime(args ...Object) (Object, error) {
	argsLen := len(args)
	if !(argsLen == 1 || argsLen == 2) {
		return nil, ErrWrongNumArguments
	}
	if _, ok := args[0].(*Time); ok {
		return args[0], nil
	}
	v, ok := ToTime(args[0])
	if ok {
		return &Time{Value: v}, nil
	}
	if argsLen == 2 {
		return args[1], nil
	}
	return UndefinedValue, nil
}

// append(arr, items...)
func builtinAppend(args ...Object) (Object, error) {
	if len(args) < 2 {
		return nil, ErrWrongNumArguments
	}
	switch arg := args[0].(type) {
	case *Array:
		return &Array{Value: append(arg.Value, args[1:]...)}, nil
	case *ImmutableArray:
		c := make([]Object, len(arg.Value)+len(args)-1)
		copy(c, arg.Value)
		copy(c[len(arg.Value):], args[1:])
		return &ImmutableArray{Value: c}, nil
	default:
		return nil, ErrInvalidArgumentType{
			Name:     "first",
			Expected: "array",
			Found:    arg.TypeName(),
		}
	}
}

// builtinDelete deletes Map keys
// usage: delete(map, "key")
// key must be a string
func builtinDelete(args ...Object) (Object, error) {
	argsLen := len(args)
	if argsLen != 2 {
		return nil, ErrWrongNumArguments
	}
	switch arg := args[0].(type) {
	case *Map:
		if key, ok := args[1].(*String); ok {
			delete(arg.Value, key.Value)
			return UndefinedValue, nil
		}
		return nil, ErrInvalidArgumentType{
			Name:     "second",
			Expected: "string",
			Found:    args[1].TypeName(),
		}
	default:
		return nil, ErrInvalidArgumentType{
			Name:     "first",
			Expected: "map",
			Found:    arg.TypeName(),
		}
	}
}

// assoc(coll, key, val) returns a new collection with the key or index set to
// val without modifying the original. Works on both mutable and immutable
// arrays (key must be an int) and maps (key must be a string). The return type
// mirrors the input type.
func builtinAssoc(args ...Object) (Object, error) {
	if len(args) != 3 {
		return nil, ErrWrongNumArguments
	}
	switch coll := args[0].(type) {
	case *Array:
		idx, ok := args[1].(Int)
		if !ok {
			return nil, ErrInvalidArgumentType{
				Name:     "second",
				Expected: "int",
				Found:    args[1].TypeName(),
			}
		}
		i := int(idx.Value)
		if i < 0 || i >= len(coll.Value) {
			return nil, ErrIndexOutOfBounds
		}
		c := make([]Object, len(coll.Value))
		copy(c, coll.Value)
		c[i] = args[2]
		return &Array{Value: c}, nil
	case *ImmutableArray:
		idx, ok := args[1].(Int)
		if !ok {
			return nil, ErrInvalidArgumentType{
				Name:     "second",
				Expected: "int",
				Found:    args[1].TypeName(),
			}
		}
		i := int(idx.Value)
		if i < 0 || i >= len(coll.Value) {
			return nil, ErrIndexOutOfBounds
		}
		c := make([]Object, len(coll.Value))
		copy(c, coll.Value)
		c[i] = args[2]
		return &ImmutableArray{Value: c}, nil
	case *Map:
		key, ok := ToString(args[1])
		if !ok {
			return nil, ErrInvalidArgumentType{
				Name:     "second",
				Expected: "string",
				Found:    args[1].TypeName(),
			}
		}
		c := make(map[string]Object, len(coll.Value)+1)
		for k, v := range coll.Value {
			c[k] = v
		}
		c[key] = args[2]
		return &Map{Value: c}, nil
	case *ImmutableMap:
		key, ok := ToString(args[1])
		if !ok {
			return nil, ErrInvalidArgumentType{
				Name:     "second",
				Expected: "string",
				Found:    args[1].TypeName(),
			}
		}
		c := make(map[string]Object, len(coll.Value)+1)
		for k, v := range coll.Value {
			c[k] = v
		}
		c[key] = args[2]
		return &ImmutableMap{Value: c}, nil
	default:
		return nil, ErrInvalidArgumentType{
			Name:     "first",
			Expected: "array/map",
			Found:    args[0].TypeName(),
		}
	}
}

// dissoc(map, key) returns a new map without the given key, without modifying
// the original. Works on both mutable and immutable maps. The return type
// mirrors the input type.
func builtinDissoc(args ...Object) (Object, error) {
	if len(args) != 2 {
		return nil, ErrWrongNumArguments
	}
	key, ok := ToString(args[1])
	if !ok {
		return nil, ErrInvalidArgumentType{
			Name:     "second",
			Expected: "string",
			Found:    args[1].TypeName(),
		}
	}
	switch m := args[0].(type) {
	case *Map:
		c := make(map[string]Object, len(m.Value))
		for k, v := range m.Value {
			if k != key {
				c[k] = v
			}
		}
		return &Map{Value: c}, nil
	case *ImmutableMap:
		c := make(map[string]Object, len(m.Value))
		for k, v := range m.Value {
			if k != key {
				c[k] = v
			}
		}
		return &ImmutableMap{Value: c}, nil
	default:
		return nil, ErrInvalidArgumentType{
			Name:     "first",
			Expected: "map",
			Found:    args[0].TypeName(),
		}
	}
}

// insert(arr, idx, val) returns a new array with val inserted before idx,
// without modifying the original. Works on both mutable and immutable arrays.
// The return type mirrors the input type. idx must be in [0, len(arr)].
func builtinInsert(args ...Object) (Object, error) {
	if len(args) != 3 {
		return nil, ErrWrongNumArguments
	}
	idx, ok := args[1].(Int)
	if !ok {
		return nil, ErrInvalidArgumentType{
			Name:     "second",
			Expected: "int",
			Found:    args[1].TypeName(),
		}
	}
	switch arr := args[0].(type) {
	case *Array:
		i := int(idx.Value)
		if i < 0 || i > len(arr.Value) {
			return nil, ErrIndexOutOfBounds
		}
		c := make([]Object, len(arr.Value)+1)
		copy(c, arr.Value[:i])
		c[i] = args[2]
		copy(c[i+1:], arr.Value[i:])
		return &Array{Value: c}, nil
	case *ImmutableArray:
		i := int(idx.Value)
		if i < 0 || i > len(arr.Value) {
			return nil, ErrIndexOutOfBounds
		}
		c := make([]Object, len(arr.Value)+1)
		copy(c, arr.Value[:i])
		c[i] = args[2]
		copy(c[i+1:], arr.Value[i:])
		return &ImmutableArray{Value: c}, nil
	default:
		return nil, ErrInvalidArgumentType{
			Name:     "first",
			Expected: "array",
			Found:    args[0].TypeName(),
		}
	}
}

// remove(arr, idx) returns a new array with the element at idx removed,
// without modifying the original. Works on both mutable and immutable arrays.
// The return type mirrors the input type.
func builtinRemove(args ...Object) (Object, error) {
	if len(args) != 2 {
		return nil, ErrWrongNumArguments
	}
	idx, ok := args[1].(Int)
	if !ok {
		return nil, ErrInvalidArgumentType{
			Name:     "second",
			Expected: "int",
			Found:    args[1].TypeName(),
		}
	}
	switch arr := args[0].(type) {
	case *Array:
		i := int(idx.Value)
		if i < 0 || i >= len(arr.Value) {
			return nil, ErrIndexOutOfBounds
		}
		c := make([]Object, len(arr.Value)-1)
		copy(c, arr.Value[:i])
		copy(c[i:], arr.Value[i+1:])
		return &Array{Value: c}, nil
	case *ImmutableArray:
		i := int(idx.Value)
		if i < 0 || i >= len(arr.Value) {
			return nil, ErrIndexOutOfBounds
		}
		c := make([]Object, len(arr.Value)-1)
		copy(c, arr.Value[:i])
		copy(c[i:], arr.Value[i+1:])
		return &ImmutableArray{Value: c}, nil
	default:
		return nil, ErrInvalidArgumentType{
			Name:     "first",
			Expected: "array",
			Found:    args[0].TypeName(),
		}
	}
}

// builtinSplice deletes and changes given Array, returns deleted items.
// usage:
// deleted_items := splice(array[,start[,delete_count[,item1[,item2[,...]]]])
func builtinSplice(args ...Object) (Object, error) {
	argsLen := len(args)
	if argsLen == 0 {
		return nil, ErrWrongNumArguments
	}

	array, ok := args[0].(*Array)
	if !ok {
		return nil, ErrInvalidArgumentType{
			Name:     "first",
			Expected: "array",
			Found:    args[0].TypeName(),
		}
	}
	arrayLen := len(array.Value)

	var startIdx int
	if argsLen > 1 {
		arg1, ok := args[1].(Int)
		if !ok {
			return nil, ErrInvalidArgumentType{
				Name:     "second",
				Expected: "int",
				Found:    args[1].TypeName(),
			}
		}
		startIdx = int(arg1.Value)
		if startIdx < 0 || startIdx > arrayLen {
			return nil, ErrIndexOutOfBounds
		}
	}

	delCount := len(array.Value)
	if argsLen > 2 {
		arg2, ok := args[2].(Int)
		if !ok {
			return nil, ErrInvalidArgumentType{
				Name:     "third",
				Expected: "int",
				Found:    args[2].TypeName(),
			}
		}
		delCount = int(arg2.Value)
		if delCount < 0 {
			return nil, ErrIndexOutOfBounds
		}
	}
	// if count of to be deleted items is bigger than expected, truncate it
	if startIdx+delCount > arrayLen {
		delCount = arrayLen - startIdx
	}
	// delete items
	endIdx := startIdx + delCount
	deleted := append([]Object{}, array.Value[startIdx:endIdx]...)

	head := array.Value[:startIdx]
	var items []Object
	if argsLen > 3 {
		items = make([]Object, 0, argsLen-3)
		for i := 3; i < argsLen; i++ {
			items = append(items, args[i])
		}
	}
	items = append(items, array.Value[endIdx:]...)
	array.Value = append(head, items...)

	// return deleted items
	return &Array{Value: deleted}, nil
}
