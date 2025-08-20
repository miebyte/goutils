package logging

import "context"

type loggingContextKey string

const (
	FieldContextKey loggingContextKey = "logging:FieldContextKey"

	LoggingGroupKey = "logging:group:key"
)

func GetContextFields(c context.Context) Fields {
	val := c.Value(FieldContextKey)
	field, ok := val.(Fields)
	if !ok {
		return make(Fields)
	}

	return field.Clone()
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

func appendGroupKey(fields Fields, group string) {
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

func GetGroupKey(fields Fields) []string {
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
