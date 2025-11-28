package ginutils

type ErrCoder interface {
	error
	Code() int
}
