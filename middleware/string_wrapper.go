package middleware

// StringWrapper is a simple wrapper for string that implements String() method
type StringWrapper string

// String implements the interface{String() string} required by the JWT validator
func (s StringWrapper) String() string {
	return string(s)
}
