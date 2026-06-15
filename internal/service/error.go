package service

// CodedError is an AWS-style error carrying an error code and message. Its
// Error string is the message. Service packages alias their local ServiceError
// to this type to share the single definition.
type CodedError struct {
	Code    string
	Message string
}

func (e *CodedError) Error() string {
	return e.Message
}
