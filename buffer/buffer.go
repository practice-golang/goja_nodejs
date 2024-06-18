package buffer

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"reflect"
	"strconv"

	"github.com/practice-golang/goja_nodejs/errors"
	"github.com/practice-golang/goja_nodejs/require"

	"github.com/dop251/base64dec"
	"github.com/grafana/sobek"
	"golang.org/x/text/encoding/unicode"
)

const ModuleName = "buffer"

type Buffer struct {
	r *sobek.Runtime

	bufferCtorObj *sobek.Object

	uint8ArrayCtorObj *sobek.Object
	uint8ArrayCtor    sobek.Constructor
}

var (
	symApi = sobek.NewSymbol("api")
)

var (
	reflectTypeArrayBuffer = reflect.TypeOf(sobek.ArrayBuffer{})
	reflectTypeString      = reflect.TypeOf("")
	reflectTypeInt         = reflect.TypeOf(int64(0))
	reflectTypeFloat       = reflect.TypeOf(0.0)
	reflectTypeBytes       = reflect.TypeOf(([]byte)(nil))
)

func Enable(runtime *sobek.Runtime) {
	runtime.Set("Buffer", require.Require(runtime, ModuleName).ToObject(runtime).Get("Buffer"))
}

func Bytes(r *sobek.Runtime, v sobek.Value) []byte {
	var b []byte
	err := r.ExportTo(v, &b)
	if err != nil {
		return []byte(v.String())
	}
	return b
}

func mod(r *sobek.Runtime) *sobek.Object {
	res := r.Get("Buffer")
	if res == nil {
		res = require.Require(r, ModuleName).ToObject(r).Get("Buffer")
	}
	m, ok := res.(*sobek.Object)
	if !ok {
		panic(r.NewTypeError("Could not extract Buffer"))
	}
	return m
}

func api(mod *sobek.Object) *Buffer {
	if s := mod.GetSymbol(symApi); s != nil {
		b, _ := s.Export().(*Buffer)
		return b
	}

	return nil
}

func GetApi(r *sobek.Runtime) *Buffer {
	return api(mod(r))
}

func DecodeBytes(r *sobek.Runtime, arg, enc sobek.Value) []byte {
	switch arg.ExportType() {
	case reflectTypeArrayBuffer:
		return arg.Export().(sobek.ArrayBuffer).Bytes()
	case reflectTypeString:
		var codec StringCodec
		if !sobek.IsUndefined(enc) {
			codec = stringCodecs[enc.String()]
		}
		if codec == nil {
			codec = utf8Codec
		}
		return codec.DecodeAppend(arg.String(), nil)
	default:
		if o, ok := arg.(*sobek.Object); ok {
			if o.ExportType() == reflectTypeBytes {
				return o.Export().([]byte)
			}
		}
	}
	panic(errors.NewTypeError(r, errors.ErrCodeInvalidArgType, "The \"data\" argument must be of type string or an instance of Buffer, TypedArray, or DataView."))
}

func WrapBytes(r *sobek.Runtime, data []byte) *sobek.Object {
	m := mod(r)
	if api := api(m); api != nil {
		return api.WrapBytes(data)
	}
	if from, ok := sobek.AssertFunction(m.Get("from")); ok {
		ab := r.NewArrayBuffer(data)
		v, err := from(m, r.ToValue(ab))
		if err != nil {
			panic(err)
		}
		return v.ToObject(r)
	}
	panic(r.NewTypeError("Buffer.from is not a function"))
}

// EncodeBytes returns the given byte slice encoded as string with the given encoding. If encoding
// is not specified or not supported, returns a Buffer that wraps the data.
func EncodeBytes(r *sobek.Runtime, data []byte, enc sobek.Value) sobek.Value {
	var codec StringCodec
	if !sobek.IsUndefined(enc) {
		codec = StringCodecByName(enc.String())
	}
	if codec != nil {
		return r.ToValue(codec.Encode(data))
	}
	return WrapBytes(r, data)
}

func (b *Buffer) WrapBytes(data []byte) *sobek.Object {
	return b.fromBytes(data)
}

func (b *Buffer) ctor(call sobek.ConstructorCall) (res *sobek.Object) {
	arg := call.Argument(0)
	switch arg.ExportType() {
	case reflectTypeInt, reflectTypeFloat:
		panic(b.r.NewTypeError("Calling the Buffer constructor with numeric argument is not implemented yet"))
		// TODO implement
	}
	return b._from(call.Arguments...)
}

type StringCodec interface {
	DecodeAppend(string, []byte) []byte
	Encode([]byte) string
}

type hexCodec struct{}

func (hexCodec) DecodeAppend(s string, b []byte) []byte {
	l := hex.DecodedLen(len(s))
	dst, res := expandSlice(b, l)
	n, err := hex.Decode(dst, []byte(s))
	if err != nil {
		res = res[:len(b)+n]
	}
	return res
}

func (hexCodec) Encode(b []byte) string {
	return hex.EncodeToString(b)
}

type _utf8Codec struct{}

func (_utf8Codec) DecodeAppend(s string, b []byte) []byte {
	r, _ := unicode.UTF8.NewEncoder().String(s)
	dst, res := expandSlice(b, len(r))
	copy(dst, r)
	return res
}

func (_utf8Codec) Encode(b []byte) string {
	r, _ := unicode.UTF8.NewDecoder().Bytes(b)
	return string(r)
}

type base64Codec struct{}

type base64UrlCodec struct {
	base64Codec
}

func (base64Codec) DecodeAppend(s string, b []byte) []byte {
	res, _ := Base64DecodeAppend(b, s)
	return res
}

func (base64Codec) Encode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func (base64UrlCodec) Encode(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

var utf8Codec StringCodec = _utf8Codec{}

var stringCodecs = map[string]StringCodec{
	"hex":       hexCodec{},
	"utf8":      utf8Codec,
	"utf-8":     utf8Codec,
	"base64":    base64Codec{},
	"base64Url": base64UrlCodec{},
}

func expandSlice(b []byte, l int) (dst, res []byte) {
	if cap(b)-len(b) < l {
		b1 := make([]byte, len(b)+l)
		copy(b1, b)
		dst = b1[len(b):]
		res = b1
	} else {
		dst = b[len(b) : len(b)+l]
		res = b[:len(b)+l]
	}
	return
}

func Base64DecodeAppend(dst []byte, src string) ([]byte, error) {
	l := base64.RawStdEncoding.DecodedLen(len(src))
	d, res := expandSlice(dst, l)
	n, err := base64dec.DecodeBase64(d, src)

	res = res[:len(dst)+n]
	return res, err
}

func (b *Buffer) fromString(str, enc string) *sobek.Object {
	codec := stringCodecs[enc]
	if codec == nil {
		codec = utf8Codec
	}
	return b.fromBytes(codec.DecodeAppend(str, nil))
}

func (b *Buffer) fromBytes(data []byte) *sobek.Object {
	o, err := b.uint8ArrayCtor(b.bufferCtorObj, b.r.ToValue(b.r.NewArrayBuffer(data)))
	if err != nil {
		panic(err)
	}
	return o
}

func (b *Buffer) _from(args ...sobek.Value) *sobek.Object {
	if len(args) == 0 {
		panic(errors.NewTypeError(b.r, errors.ErrCodeInvalidArgType, "The first argument must be of type string or an instance of Buffer, ArrayBuffer, or Array or an Array-like Object. Received undefined"))
	}
	arg := args[0]
	switch arg.ExportType() {
	case reflectTypeArrayBuffer:
		v, err := b.uint8ArrayCtor(b.bufferCtorObj, args...)
		if err != nil {
			panic(err)
		}
		return v
	case reflectTypeString:
		var enc string
		if len(args) > 1 {
			enc = args[1].String()
		}
		return b.fromString(arg.String(), enc)
	default:
		if o, ok := arg.(*sobek.Object); ok {
			if o.ExportType() == reflectTypeBytes {
				bb, _ := o.Export().([]byte)
				a := make([]byte, len(bb))
				copy(a, bb)
				return b.fromBytes(a)
			} else {
				if f, ok := sobek.AssertFunction(o.Get("valueOf")); ok {
					valueOf, err := f(o)
					if err != nil {
						panic(err)
					}
					if valueOf != o {
						args[0] = valueOf
						return b._from(args...)
					}
				}

				if s := o.GetSymbol(sobek.SymToPrimitive); s != nil {
					if f, ok := sobek.AssertFunction(s); ok {
						str, err := f(o, b.r.ToValue("string"))
						if err != nil {
							panic(err)
						}
						args[0] = str
						return b._from(args...)
					}
				}
			}
			// array-like
			if v := o.Get("length"); v != nil {
				length := int(v.ToInteger())
				a := make([]byte, length)
				for i := 0; i < length; i++ {
					item := o.Get(strconv.Itoa(i))
					if item != nil {
						a[i] = byte(item.ToInteger())
					}
				}
				return b.fromBytes(a)
			}
		}
	}
	panic(errors.NewTypeError(b.r, errors.ErrCodeInvalidArgType, "The first argument must be of type string or an instance of Buffer, ArrayBuffer, or Array or an Array-like Object. Received %s", arg))
}

func (b *Buffer) from(call sobek.FunctionCall) sobek.Value {
	return b._from(call.Arguments...)
}

func isNumber(v sobek.Value) bool {
	switch v.ExportType() {
	case reflectTypeInt, reflectTypeFloat:
		return true
	}
	return false
}

func isString(v sobek.Value) bool {
	return v.ExportType() == reflectTypeString
}

func StringCodecByName(name string) StringCodec {
	return stringCodecs[name]
}

func (b *Buffer) getStringCodec(enc sobek.Value) (codec StringCodec) {
	if !sobek.IsUndefined(enc) {
		codec = stringCodecs[enc.String()]
		if codec == nil {
			panic(errors.NewTypeError(b.r, "ERR_UNKNOWN_ENCODING", "Unknown encoding: %s", enc))
		}
	} else {
		codec = utf8Codec
	}
	return
}

func (b *Buffer) fill(buf []byte, fill string, enc sobek.Value) []byte {
	codec := b.getStringCodec(enc)
	b1 := codec.DecodeAppend(fill, buf[:0])
	if len(b1) > len(buf) {
		return b1[:len(buf)]
	}
	for i := len(b1); i < len(buf); {
		i += copy(buf[i:], buf[:i])
	}
	return buf
}

func (b *Buffer) alloc(call sobek.FunctionCall) sobek.Value {
	arg0 := call.Argument(0)
	size := -1
	if isNumber(arg0) {
		size = int(arg0.ToInteger())
	}
	if size < 0 {
		panic(errors.NewTypeError(b.r, errors.ErrCodeInvalidArgType, "The \"size\" argument must be of type number."))
	}
	fill := call.Argument(1)
	buf := make([]byte, size)
	if !sobek.IsUndefined(fill) {
		if isString(fill) {
			var enc sobek.Value
			if a := call.Argument(2); isString(a) {
				enc = a
			} else {
				enc = sobek.Undefined()
			}
			buf = b.fill(buf, fill.String(), enc)
		} else {
			fill = fill.ToNumber()
			if !sobek.IsNaN(fill) && !sobek.IsInfinity(fill) {
				fillByte := byte(fill.ToInteger())
				if fillByte != 0 {
					for i := range buf {
						buf[i] = fillByte
					}
				}
			}
		}
	}
	return b.fromBytes(buf)
}

func (b *Buffer) proto_toString(call sobek.FunctionCall) sobek.Value {
	bb := Bytes(b.r, call.This)
	codec := b.getStringCodec(call.Argument(0))
	return b.r.ToValue(codec.Encode(bb))
}

func (b *Buffer) proto_equals(call sobek.FunctionCall) sobek.Value {
	bb := Bytes(b.r, call.This)
	other := call.Argument(0)
	if b.r.InstanceOf(other, b.uint8ArrayCtorObj) {
		otherBytes := Bytes(b.r, other)
		return b.r.ToValue(bytes.Equal(bb, otherBytes))
	}
	panic(errors.NewTypeError(b.r, errors.ErrCodeInvalidArgType, "The \"otherBuffer\" argument must be an instance of Buffer or Uint8Array."))
}

func Require(runtime *sobek.Runtime, module *sobek.Object) {
	b := &Buffer{r: runtime}
	uint8Array := runtime.Get("Uint8Array")
	if c, ok := sobek.AssertConstructor(uint8Array); ok {
		b.uint8ArrayCtor = c
	} else {
		panic(runtime.NewTypeError("Uint8Array is not a constructor"))
	}
	uint8ArrayObj := uint8Array.ToObject(runtime)

	ctor := runtime.ToValue(b.ctor).ToObject(runtime)
	ctor.SetPrototype(uint8ArrayObj)
	ctor.DefineDataPropertySymbol(symApi, runtime.ToValue(b), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_FALSE)
	b.bufferCtorObj = ctor
	b.uint8ArrayCtorObj = uint8ArrayObj

	proto := runtime.NewObject()
	proto.SetPrototype(uint8ArrayObj.Get("prototype").ToObject(runtime))
	proto.DefineDataProperty("constructor", ctor, sobek.FLAG_TRUE, sobek.FLAG_TRUE, sobek.FLAG_FALSE)
	proto.Set("equals", b.proto_equals)
	proto.Set("toString", b.proto_toString)

	ctor.Set("prototype", proto)
	ctor.Set("poolSize", 8192)
	ctor.Set("from", b.from)
	ctor.Set("alloc", b.alloc)

	exports := module.Get("exports").(*sobek.Object)
	exports.Set("Buffer", ctor)
}

func init() {
	require.RegisterCoreModule(ModuleName, Require)
}
