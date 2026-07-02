package logging

import "context"

type loggingContextKey string

const (
	FieldContextKey loggingContextKey = "logging:FieldContextKey"

	LoggingGroupKey = "logging:group:key"
)

func GetContextFields(c context.Context) CtxFields {
	val := c.Value(FieldContextKey)
	field, ok := val.(CtxFields)
	if !ok {
		return make(CtxFields)
	}

	return field.Clone()
}

func copyContextFields(dst CtxFields, c context.Context) CtxFields {
	val := c.Value(FieldContextKey)
	fields, ok := val.(CtxFields)
	if dst == nil {
		dst = make(CtxFields, len(fields))
	} else {
		clear(dst)
	}

	if !ok || len(fields) == 0 {
		return dst
	}

	for key, value := range fields {
		dst[key] = value
	}

	return dst
}

func CloneContextFields(c context.Context) context.Context {
	fields := GetContextFields(c)

	return context.WithValue(context.TODO(), FieldContextKey, fields)
}

func With(c context.Context, key string, values ...any) context.Context {
	if c == nil {
		c = context.TODO()
	}

	if key == "" && len(values) == 0 {
		return c
	}

	fields := GetContextFields(c)
	newF := fields.Clone()

	if len(values) == 0 {
		appendGroupKey(newF, key)
	} else {
		newF[key] = values[0]
	}

	return context.WithValue(c, FieldContextKey, newF)
}

func appendGroupKey(fields CtxFields, group string) {
	var groupList []string
	value, exists := fields[LoggingGroupKey]
	if !exists {
		groupList = make([]string, 0)
	} else {
		groupList = value.([]string)
	}

	groupList = append(groupList, group)
	fields[LoggingGroupKey] = groupList
}

func GetGroupKey(fields CtxFields) []string {
	if fields == nil {
		return nil
	}

	value, ok := fields[LoggingGroupKey]
	if !ok {
		return nil
	}

	delete(fields, LoggingGroupKey)
	return value.([]string)
}
