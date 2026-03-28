// Package types provides shared type definitions for Cortex.
package types

// Result represents a generic result with value and error.
type Result[T any] struct {
	Value T
	Err   error
}

// Ok creates a successful result.
func Ok[T any](value T) Result[T] {
	return Result[T]{Value: value}
}

// Err creates a failed result.
func Err[T any](err error) Result[T] {
	return Result[T]{Err: err}
}

// IsOk returns true if the result is successful.
func (r Result[T]) IsOk() bool {
	return r.Err == nil
}

// IsErr returns true if the result is an error.
func (r Result[T]) IsErr() bool {
	return r.Err != nil
}

// Unwrap returns the value or panics if error.
func (r Result[T]) Unwrap() T {
	if r.Err != nil {
		panic(r.Err)
	}
	return r.Value
}

// UnwrapOr returns the value or the default if error.
func (r Result[T]) UnwrapOr(defaultValue T) T {
	if r.Err != nil {
		return defaultValue
	}
	return r.Value
}

// Option represents an optional value.
type Option[T any] struct {
	value *T
}

// Some creates an Option with a value.
func Some[T any](value T) Option[T] {
	return Option[T]{value: &value}
}

// None creates an empty Option.
func None[T any]() Option[T] {
	return Option[T]{value: nil}
}

// IsSome returns true if the Option has a value.
func (o Option[T]) IsSome() bool {
	return o.value != nil
}

// IsNone returns true if the Option is empty.
func (o Option[T]) IsNone() bool {
	return o.value == nil
}

// Unwrap returns the value or panics if empty.
func (o Option[T]) Unwrap() T {
	if o.value == nil {
		panic("called Unwrap on None")
	}
	return *o.value
}

// UnwrapOr returns the value or the default if empty.
func (o Option[T]) UnwrapOr(defaultValue T) T {
	if o.value == nil {
		return defaultValue
	}
	return *o.value
}

// Ptr returns a pointer to the value, or nil if empty.
func (o Option[T]) Ptr() *T {
	return o.value
}

// Map transforms the Option value if present.
func Map[T, U any](o Option[T], fn func(T) U) Option[U] {
	if o.IsNone() {
		return None[U]()
	}
	return Some(fn(o.Unwrap()))
}

// Pair represents a key-value pair.
type Pair[K, V any] struct {
	Key   K
	Value V
}

// NewPair creates a new Pair.
func NewPair[K, V any](key K, value V) Pair[K, V] {
	return Pair[K, V]{Key: key, Value: value}
}

// Pagination contains pagination parameters.
type Pagination struct {
	Offset int
	Limit  int
}

// DefaultPagination returns default pagination.
func DefaultPagination() Pagination {
	return Pagination{
		Offset: 0,
		Limit:  100,
	}
}

// SortOrder represents sort direction.
type SortOrder string

const (
	SortAsc  SortOrder = "asc"
	SortDesc SortOrder = "desc"
)

// ListOptions contains common list options.
type ListOptions struct {
	Pagination
	SortBy    string
	SortOrder SortOrder
}

// DefaultListOptions returns default list options.
func DefaultListOptions() ListOptions {
	return ListOptions{
		Pagination: DefaultPagination(),
		SortBy:     "",
		SortOrder:  SortAsc,
	}
}
