package logging

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"time"
)

// A FieldType indicates which member of the Field union struct should be used
// and how it should be serialized.
type FieldType uint8

const (
	// UnknownType is the default field type. Attempting to add it to an encoder will panic.
	UnknownType FieldType = iota
	// BoolType indicates that the field carries a bool.
	BoolType
	// DurationType indicates that the field carries a time.Duration.
	DurationType
	// Float64Type indicates that the field carries a float64.
	Float64Type
	// Float32Type indicates that the field carries a float32.
	Float32Type
	// Int64Type indicates that the field carries an int64.
	Int64Type
	// Int32Type indicates that the field carries an int32.
	Int32Type
	// Int16Type indicates that the field carries an int16.
	Int16Type
	// Int8Type indicates that the field carries an int8.
	Int8Type
	// StringType indicates that the field carries a string.
	StringType
	// TimeType indicates that the field carries a time.Time that is
	// representable by a UnixNano() stored as an int64.
	TimeType
	// TimeFullType indicates that the field carries a time.Time stored as-is.
	TimeFullType
	// Uint64Type indicates that the field carries a uint64.
	Uint64Type
	// Uint32Type indicates that the field carries a uint32.
	Uint32Type
	// Uint16Type indicates that the field carries a uint16.
	Uint16Type
	// Uint8Type indicates that the field carries a uint8.
	Uint8Type
	// UintptrType indicates that the field carries a uintptr.
	UintptrType
	// StringerType indicates that the field carries a fmt.Stringer.
	StringerType
	// ErrorType indicates that the field carries an error.
	ErrorType
	// AnyType indicates that the field carries an arbitrary value.
	AnyType
	// SkipType indicates that the field is a no-op.
	SkipType
)

type Field struct {
	Key       string
	Type      FieldType
	Integer   int64
	String    string
	Interface any
}

// AddTo 将字段追加到缓冲区中。
func (f *Field) AddTo(buf *bytes.Buffer) {
	if f == nil || f.Type == SkipType || f.Key == "" {
		return
	}

	buf.WriteString(f.Key)
	buf.WriteByte('=')

	switch f.Type {
	case BoolType:
		buf.WriteString(strconv.FormatBool(f.Integer == 1))
	case DurationType:
		buf.WriteString(time.Duration(f.Integer).String())
	case Float64Type:
		buf.WriteString(strconv.FormatFloat(math.Float64frombits(uint64(f.Integer)), 'f', -1, 64))
	case Float32Type:
		buf.WriteString(strconv.FormatFloat(float64(math.Float32frombits(uint32(f.Integer))), 'f', -1, 32))
	case Int64Type, Int32Type, Int16Type, Int8Type:
		buf.WriteString(strconv.FormatInt(f.Integer, 10))
	case StringType:
		buf.WriteString(f.String)
	case TimeType:
		buf.WriteString(strconv.FormatInt(f.Integer, 10))
	case TimeFullType:
		t, ok := f.Interface.(time.Time)
		if !ok {
			_, _ = fmt.Fprint(buf, f.Interface)
			break
		}
		buf.WriteString(t.Format(time.RFC3339Nano))
	case Uint64Type, Uint32Type, Uint16Type, Uint8Type, UintptrType:
		buf.WriteString(strconv.FormatUint(uint64(f.Integer), 10))
	case StringerType, ErrorType, AnyType:
		_, _ = fmt.Fprint(buf, f.Interface)
	default:
		_, _ = fmt.Fprint(buf, f.Interface)
	}

	buf.WriteByte(' ')
}

func Skip() Field {
	return Field{Type: SkipType}
}

func Bool(key string, value bool) Field {
	if value {
		return Field{Key: key, Type: BoolType, Integer: 1}
	}

	return Field{Key: key, Type: BoolType}
}

func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Type: DurationType, Integer: int64(value)}
}

func Float64(key string, value float64) Field {
	return Field{Key: key, Type: Float64Type, Integer: int64(math.Float64bits(value))}
}

func Float32(key string, value float32) Field {
	return Field{Key: key, Type: Float32Type, Integer: int64(math.Float32bits(value))}
}

func Int(key string, value int) Field {
	return Field{Key: key, Type: Int64Type, Integer: int64(value)}
}

func Int64(key string, value int64) Field {
	return Field{Key: key, Type: Int64Type, Integer: value}
}

func Int32(key string, value int32) Field {
	return Field{Key: key, Type: Int32Type, Integer: int64(value)}
}

func Int16(key string, value int16) Field {
	return Field{Key: key, Type: Int16Type, Integer: int64(value)}
}

func Int8(key string, value int8) Field {
	return Field{Key: key, Type: Int8Type, Integer: int64(value)}
}

func String(key string, value string) Field {
	return Field{Key: key, Type: StringType, String: value}
}

func Time(key string, value time.Time) Field {
	return Field{Key: key, Type: TimeType, Integer: value.UnixMilli()}
}

func TimeFull(key string, value time.Time) Field {
	return Field{Key: key, Type: TimeFullType, Interface: value}
}

func Uint(key string, value uint) Field {
	return Field{Key: key, Type: Uint64Type, Integer: int64(value)}
}

func Uint64(key string, value uint64) Field {
	return Field{Key: key, Type: Uint64Type, Integer: int64(value)}
}

func Uint32(key string, value uint32) Field {
	return Field{Key: key, Type: Uint32Type, Integer: int64(value)}
}

func Uint16(key string, value uint16) Field {
	return Field{Key: key, Type: Uint16Type, Integer: int64(value)}
}

func Uint8(key string, value uint8) Field {
	return Field{Key: key, Type: Uint8Type, Integer: int64(value)}
}

func Uintptr(key string, value uintptr) Field {
	return Field{Key: key, Type: UintptrType, Integer: int64(value)}
}

func Stringer(key string, value fmt.Stringer) Field {
	if value == nil {
		return Skip()
	}

	return Field{Key: key, Type: StringerType, Interface: value}
}

func Err(err error) Field {
	if err == nil {
		return Skip()
	}

	return Field{Key: "error", Type: ErrorType, Interface: err}
}

func NamedError(key string, err error) Field {
	if err == nil {
		return Skip()
	}

	return Field{Key: key, Type: ErrorType, Interface: err}
}

func Any(key string, value any) Field {
	switch v := value.(type) {
	case nil:
		return Skip()
	case Field:
		return v
	case bool:
		return Bool(key, v)
	case time.Duration:
		return Duration(key, v)
	case float64:
		return Float64(key, v)
	case float32:
		return Float32(key, v)
	case int:
		return Int(key, v)
	case int64:
		return Int64(key, v)
	case int32:
		return Int32(key, v)
	case int16:
		return Int16(key, v)
	case int8:
		return Int8(key, v)
	case string:
		return String(key, v)
	case time.Time:
		return Time(key, v)
	case uint:
		return Uint(key, v)
	case uint64:
		return Uint64(key, v)
	case uint32:
		return Uint32(key, v)
	case uint16:
		return Uint16(key, v)
	case uint8:
		return Uint8(key, v)
	case uintptr:
		return Uintptr(key, v)
	case error:
		return NamedError(key, v)
	case fmt.Stringer:
		return Stringer(key, v)
	default:
		return Field{Key: key, Type: AnyType, Interface: value}
	}

}
