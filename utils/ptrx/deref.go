package ptrx

func TypesValue[T any](a *T) T {
	var zeroT T
	if a == nil {
		return zeroT
	}

	return *a
}

func StringValue(a *string) string {
	if a == nil {
		return ""
	}
	return *a
}

func IntValue(a *int) int {
	if a == nil {
		return 0
	}
	return *a
}

func Int8Value(a *int8) int8 {
	if a == nil {
		return 0
	}
	return *a
}

func Int16Value(a *int16) int16 {
	if a == nil {
		return 0
	}
	return *a
}

func Int32Value(a *int32) int32 {
	if a == nil {
		return 0
	}
	return *a
}

func Int64Value(a *int64) int64 {
	if a == nil {
		return 0
	}
	return *a
}

func BoolValue(a *bool) bool {
	if a == nil {
		return false
	}
	return *a
}

func UintValue(a *uint) uint {
	if a == nil {
		return 0
	}
	return *a
}

func Uint8Value(a *uint8) uint8 {
	if a == nil {
		return 0
	}
	return *a
}

func Uint16Value(a *uint16) uint16 {
	if a == nil {
		return 0
	}
	return *a
}

func Uint32Value(a *uint32) uint32 {
	if a == nil {
		return 0
	}
	return *a
}

func Uint64Value(a *uint64) uint64 {
	if a == nil {
		return 0
	}
	return *a
}

func Float32Value(a *float32) float32 {
	if a == nil {
		return 0
	}
	return *a
}

func Float64Value(a *float64) float64 {
	if a == nil {
		return 0
	}
	return *a
}

func IntValueSlice(a []*int) []int {
	if a == nil {
		return nil
	}
	res := make([]int, len(a))
	for i := range len(a) {
		if a[i] != nil {
			res[i] = *a[i]
		}
	}
	return res
}

func Int8ValueSlice(a []*int8) []int8 {
	if a == nil {
		return nil
	}
	res := make([]int8, len(a))
	for i := range len(a) {
		if a[i] != nil {
			res[i] = *a[i]
		}
	}
	return res
}

func Int16ValueSlice(a []*int16) []int16 {
	if a == nil {
		return nil
	}
	res := make([]int16, len(a))
	for i := range len(a) {
		if a[i] != nil {
			res[i] = *a[i]
		}
	}
	return res
}

func Int32ValueSlice(a []*int32) []int32 {
	if a == nil {
		return nil
	}
	res := make([]int32, len(a))
	for i := range len(a) {
		if a[i] != nil {
			res[i] = *a[i]
		}
	}
	return res
}

func Int64ValueSlice(a []*int64) []int64 {
	if a == nil {
		return nil
	}
	res := make([]int64, len(a))
	for i := range len(a) {
		if a[i] != nil {
			res[i] = *a[i]
		}
	}
	return res
}

func UintValueSlice(a []*uint) []uint {
	if a == nil {
		return nil
	}
	res := make([]uint, len(a))
	for i := range len(a) {
		if a[i] != nil {
			res[i] = *a[i]
		}
	}
	return res
}

func Uint8ValueSlice(a []*uint8) []uint8 {
	if a == nil {
		return nil
	}
	res := make([]uint8, len(a))
	for i := range len(a) {
		if a[i] != nil {
			res[i] = *a[i]
		}
	}
	return res
}

func Uint16ValueSlice(a []*uint16) []uint16 {
	if a == nil {
		return nil
	}
	res := make([]uint16, len(a))
	for i := range len(a) {
		if a[i] != nil {
			res[i] = *a[i]
		}
	}
	return res
}

func Uint32ValueSlice(a []*uint32) []uint32 {
	if a == nil {
		return nil
	}
	res := make([]uint32, len(a))
	for i := range len(a) {
		if a[i] != nil {
			res[i] = *a[i]
		}
	}
	return res
}

func Uint64ValueSlice(a []*uint64) []uint64 {
	if a == nil {
		return nil
	}
	res := make([]uint64, len(a))
	for i := range len(a) {
		if a[i] != nil {
			res[i] = *a[i]
		}
	}
	return res
}

func Float32ValueSlice(a []*float32) []float32 {
	if a == nil {
		return nil
	}
	res := make([]float32, len(a))
	for i := range len(a) {
		if a[i] != nil {
			res[i] = *a[i]
		}
	}
	return res
}

func Float64ValueSlice(a []*float64) []float64 {
	if a == nil {
		return nil
	}
	res := make([]float64, len(a))
	for i := range len(a) {
		if a[i] != nil {
			res[i] = *a[i]
		}
	}
	return res
}

func StringSliceValue(a []*string) []string {
	if a == nil {
		return nil
	}
	res := make([]string, len(a))
	for i := range len(a) {
		if a[i] != nil {
			res[i] = *a[i]
		}
	}
	return res
}

func BoolSliceValue(a []*bool) []bool {
	if a == nil {
		return nil
	}
	res := make([]bool, len(a))
	for i := range len(a) {
		if a[i] != nil {
			res[i] = *a[i]
		}
	}
	return res
}
