package tengo

import "time"

// ArgCount returns ErrWrongNumArguments if len(args) != n.
func ArgCount(args []Object, n int) error {
	if len(args) != n {
		return ErrWrongNumArguments
	}
	return nil
}

// ArgCountRange returns ErrWrongNumArguments if len(args) is outside [min, max].
func ArgCountRange(args []Object, min, max int) error {
	if len(args) < min || len(args) > max {
		return ErrWrongNumArguments
	}
	return nil
}

// ArgCountAtLeast returns ErrWrongNumArguments if len(args) < min.
func ArgCountAtLeast(args []Object, min int) error {
	if len(args) < min {
		return ErrWrongNumArguments
	}
	return nil
}

// ArgString extracts a string-compatible value from args[i].
func ArgString(args []Object, i int, name string) (string, error) {
	s, ok := ToString(args[i])
	if !ok {
		return "", ErrInvalidArgumentType{
			Name:     name,
			Expected: "string(compatible)",
			Found:    args[i].TypeName(),
		}
	}
	return s, nil
}

// ArgInt extracts an int64 from args[i].
func ArgInt(args []Object, i int, name string) (int64, error) {
	n, ok := ToInt64(args[i])
	if !ok {
		return 0, ErrInvalidArgumentType{
			Name:     name,
			Expected: "int(compatible)",
			Found:    args[i].TypeName(),
		}
	}
	return n, nil
}

// ArgFloat extracts a float64 from args[i].
func ArgFloat(args []Object, i int, name string) (float64, error) {
	f, ok := ToFloat64(args[i])
	if !ok {
		return 0, ErrInvalidArgumentType{
			Name:     name,
			Expected: "float(compatible)",
			Found:    args[i].TypeName(),
		}
	}
	return f, nil
}

// ArgBool extracts a bool from args[i].
func ArgBool(args []Object, i int, name string) (bool, error) {
	b, ok := ToBool(args[i])
	if !ok {
		return false, ErrInvalidArgumentType{
			Name:     name,
			Expected: "bool",
			Found:    args[i].TypeName(),
		}
	}
	return b, nil
}

// ArgBytes extracts a []byte from args[i].
func ArgBytes(args []Object, i int, name string) ([]byte, error) {
	b, ok := ToByteSlice(args[i])
	if !ok {
		return nil, ErrInvalidArgumentType{
			Name:     name,
			Expected: "bytes",
			Found:    args[i].TypeName(),
		}
	}
	return b, nil
}

// ArgTime extracts a time.Time from args[i].
func ArgTime(args []Object, i int, name string) (time.Time, error) {
	t, ok := ToTime(args[i])
	if !ok {
		return time.Time{}, ErrInvalidArgumentType{
			Name:     name,
			Expected: "time",
			Found:    args[i].TypeName(),
		}
	}
	return t, nil
}
