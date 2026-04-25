package stdlib

import (
	"bytes"
	gojson "encoding/json"

	"github.com/tengolang/tengo/v3"
	"github.com/tengolang/tengo/v3/stdlib/json"
)

var jsonModule = map[string]tengo.Object{
	"decode": &tengo.UserFunction{
		Name:  "decode",
		Value: jsonDecode,
	},
	"encode": &tengo.UserFunction{
		Name:  "encode",
		Value: jsonEncode,
	},
	"indent": &tengo.UserFunction{
		Name:  "encode",
		Value: jsonIndent,
	},
	"html_escape": &tengo.UserFunction{
		Name:  "html_escape",
		Value: jsonHTMLEscape,
	},
}

func jsonDecode(args ...tengo.Object) (ret tengo.Object, err error) {
	if len(args) != 1 {
		return nil, tengo.ErrWrongNumArguments
	}

	switch o := args[0].(type) {
	case *tengo.Bytes:
		v, err := json.Decode(o.Value)
		if err != nil {
			return &tengo.Error{
				Value: &tengo.String{Value: err.Error()},
			}, nil
		}
		return v, nil
	default:
		if sv, ok := tengo.StringValue(args[0]); ok {
			v, err := json.Decode([]byte(sv))
			if err != nil {
				return &tengo.Error{Value: &tengo.String{Value: err.Error()}}, nil
			}
			return v, nil
		}
		return nil, tengo.ErrInvalidArgumentType{
			Name:     "first",
			Expected: "bytes/string",
			Found:    args[0].TypeName(),
		}
	}
}

func jsonEncode(args ...tengo.Object) (ret tengo.Object, err error) {
	if len(args) != 1 {
		return nil, tengo.ErrWrongNumArguments
	}

	b, err := json.Encode(args[0])
	if err != nil {
		return &tengo.Error{Value: &tengo.String{Value: err.Error()}}, nil
	}

	return &tengo.Bytes{Value: b}, nil
}

func jsonIndent(args ...tengo.Object) (ret tengo.Object, err error) {
	if len(args) != 3 {
		return nil, tengo.ErrWrongNumArguments
	}

	prefix, ok := tengo.ToString(args[1])
	if !ok {
		return nil, tengo.ErrInvalidArgumentType{
			Name:     "prefix",
			Expected: "string(compatible)",
			Found:    args[1].TypeName(),
		}
	}

	indent, ok := tengo.ToString(args[2])
	if !ok {
		return nil, tengo.ErrInvalidArgumentType{
			Name:     "indent",
			Expected: "string(compatible)",
			Found:    args[2].TypeName(),
		}
	}

	switch o := args[0].(type) {
	case *tengo.Bytes:
		var dst bytes.Buffer
		err := gojson.Indent(&dst, o.Value, prefix, indent)
		if err != nil {
			return &tengo.Error{
				Value: &tengo.String{Value: err.Error()},
			}, nil
		}
		return &tengo.Bytes{Value: dst.Bytes()}, nil
	default:
		if sv, ok := tengo.StringValue(args[0]); ok {
			var dst bytes.Buffer
			if err := gojson.Indent(&dst, []byte(sv), prefix, indent); err != nil {
				return &tengo.Error{Value: &tengo.String{Value: err.Error()}}, nil
			}
			return &tengo.Bytes{Value: dst.Bytes()}, nil
		}
		return nil, tengo.ErrInvalidArgumentType{
			Name:     "first",
			Expected: "bytes/string",
			Found:    args[0].TypeName(),
		}
	}
}

func jsonHTMLEscape(args ...tengo.Object) (ret tengo.Object, err error) {
	if len(args) != 1 {
		return nil, tengo.ErrWrongNumArguments
	}

	switch o := args[0].(type) {
	case *tengo.Bytes:
		var dst bytes.Buffer
		gojson.HTMLEscape(&dst, o.Value)
		return &tengo.Bytes{Value: dst.Bytes()}, nil
	default:
		if sv, ok := tengo.StringValue(args[0]); ok {
			var dst bytes.Buffer
			gojson.HTMLEscape(&dst, []byte(sv))
			return &tengo.Bytes{Value: dst.Bytes()}, nil
		}
		return nil, tengo.ErrInvalidArgumentType{
			Name:     "first",
			Expected: "bytes/string",
			Found:    args[0].TypeName(),
		}
	}
}
