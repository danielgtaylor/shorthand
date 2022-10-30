package shorthand

import "fmt"

// Error represents an error at a specific location.
type Error interface {
	Error() string

	// Offset returns the character offset of the error within the experssion.
	Offset() uint

	// Length returns the length in bytes after the offset where the error ends.
	Length() uint

	// Pretty prints out a message with a pointer to the source location of the
	// error.
	Pretty() string
}

type exprErr struct {
	source  *string
	offset  uint
	length  uint
	message string
}

func (e *exprErr) Error() string {
	return e.message
}

func (e *exprErr) Offset() uint {
	return e.offset
}

func (e *exprErr) Length() uint {
	return e.length
}

func (e *exprErr) Pretty() string {
	// TODO: find previous line break if exists, also truncate to e.g. 80 chars, show one line only. Make it dead simple!
	msg := e.Error() + "\n" + *e.source + "\n"
	for i := uint(0); i < e.offset; i++ {
		msg += "."
	}
	for i := uint(0); i < e.length; i++ {
		msg += "^"
	}
	return msg
}

// NewError creates a new error at a specific location.
func NewError(source *string, offset uint, length uint, format string, a ...interface{}) Error {
	if length < 1 {
		length = 1
	}
	return &exprErr{
		source:  source,
		offset:  offset,
		length:  length,
		message: fmt.Sprintf(format, a...),
	}
}
