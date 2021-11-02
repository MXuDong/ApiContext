package api

import "fmt"

type ErrorType string

var (
	Unknown_ErrorType         ErrorType = "Unknown"
	TypeError_ErrorType       ErrorType = "Type error"
	OutOfDeadLine_ErrorType   ErrorType = "Out of dead line"
	ParamCountError_ErrorType ErrorType = "Param count error"
	ParamTypeError_ErrorType  ErrorType = "Param(s) type error"
)

func NewApiError(info string, object interface{}, errType ErrorType) *ApiError {
	if len(errType) == 0 {
		errType = Unknown_ErrorType
	}

	return &ApiError{
		ErrorInfo:   info,
		ErrorObject: object,
		ErrorType:   errType,
	}
}

type ApiError struct {
	ErrorOrigin error

	ErrorInfo   string
	ErrorObject interface{}

	LastLevelError *ApiError // ParentError

	ErrorType ErrorType
}

func (ae *ApiError) String() string {
	// implement me
	return ""
}

func (ae *ApiError) Error() string {
	// implement me
	if ae.ErrorOrigin == nil {
		ae.ErrorOrigin = fmt.Errorf(ae.ErrorInfo+";Obj: %v", ae.ErrorObject)
	}

	if ae.LastLevelError != nil {
		return ae.LastLevelError.Error() + "\n" + ae.ErrorOrigin.Error()
	}

	return ae.ErrorOrigin.Error()
}

func (ae *ApiError) WithStruck(info string, object interface{}, errType ErrorType) *ApiError {
	err := &ApiError{
		ErrorInfo:   info,
		ErrorObject: object,
		ErrorType:   Unknown_ErrorType,
	}

	if len(errType) != 0 {
		err.ErrorType = errType
	}

	return err
}
