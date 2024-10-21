package jsonrpc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// Define the error types
type StaticError struct{}

func (e *StaticError) Error() string { return "static error" }

type SimpleError struct {
	Message string
}

func (e *SimpleError) Error() string {
	return e.Message
}

func (e *SimpleError) FromJSONRPCError(jerr JSONRPCError) error {
	e.Message = jerr.Message
	return nil
}

func (e *SimpleError) ToJSONRPCError() (JSONRPCError, error) {
	return JSONRPCError{Message: e.Message}, nil
}

var _ ConvertableJSONRPCError = (*SimpleError)(nil)

type DataStringError struct {
	Message string `json:"message"`
	Data    string `json:"data"`
}

func (e *DataStringError) Error() string {
	return e.Message
}

func (e *DataStringError) ErrorData() any {
	return e.Data
}

func (e *DataStringError) FromJSONRPCError(jerr JSONRPCError) error {
	e.Message = jerr.Message
	if err := json.Unmarshal(jerr.Data, &e.Data); err != nil {
		return err
	}
	return nil
}

func (e *DataStringError) ToJSONRPCError() (JSONRPCError, error) {
	data, err := json.Marshal(e.Data)
	if err != nil {
		return JSONRPCError{}, err
	}
	return JSONRPCError{Message: e.Message, Data: data}, nil
}

var _ ConvertableJSONRPCError = (*DataStringError)(nil)

type DataComplexError struct {
	Message      string
	internalData ComplexData
}

func (e *DataComplexError) Error() string {
	return e.Message
}

func (e *DataComplexError) ErrorData() any {
	return e.internalData
}

func (e *DataComplexError) FromJSONRPCError(jerr JSONRPCError) error {
	e.Message = jerr.Message
	if err := json.Unmarshal(jerr.Data, &e.internalData); err != nil {
		return err
	}
	return nil
}

func (e *DataComplexError) ToJSONRPCError() (JSONRPCError, error) {
	data, err := json.Marshal(e.internalData)
	if err != nil {
		return JSONRPCError{}, err
	}
	return JSONRPCError{Message: e.Message, Data: data}, nil
}

var _ ConvertableJSONRPCError = (*DataComplexError)(nil)

type MetaError struct {
	Message string
	Details string
}

func (e *MetaError) Error() string {
	return e.Message
}

func (e *MetaError) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Message string `json:"message"`
		Details string `json:"details"`
	}{
		Message: e.Message,
		Details: e.Details,
	})
}

func (e *MetaError) UnmarshalJSON(data []byte) error {
	var temp struct {
		Message string `json:"message"`
		Details string `json:"details"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	e.Message = temp.Message
	e.Details = temp.Details
	return nil
}

type ComplexError struct {
	Message string
	Data    ComplexData
	Details string
}

func (e *ComplexError) Error() string {
	return e.Message
}

func (e *ComplexError) ErrorData() any {
	return e.Data
}

func (e *ComplexError) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Message string `json:"message"`
		Details string `json:"details"`
		Data    any    `json:"data"`
	}{
		Details: e.Details,
		Message: e.Message,
		Data:    e.Data,
	})
}

func (e *ComplexError) UnmarshalJSON(data []byte) error {
	var temp struct {
		Message string      `json:"message"`
		Details string      `json:"details"`
		Data    ComplexData `json:"data"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	e.Details = temp.Details
	e.Message = temp.Message
	e.Data = temp.Data
	return nil
}

type ComplexData struct {
	Foo string `json:"foo"`
	Bar int    `json:"bar"`
}

func TestRespErrorVal(t *testing.T) {
	// Initialize the Errors struct and register error types
	errorsMap := NewErrors()
	errorsMap.Register(1000, new(*StaticError))
	errorsMap.Register(1001, new(*SimpleError))
	errorsMap.Register(1002, new(*DataStringError))
	errorsMap.Register(1003, new(*DataComplexError))
	errorsMap.Register(1004, new(*MetaError))
	errorsMap.Register(1005, new(*ComplexError))

	// Define test cases
	testCases := []struct {
		name            string
		respError       *JSONRPCError
		expectedType    interface{}
		expectedMessage string
		verify          func(t *testing.T, err error)
	}{
		{
			name: "StaticError",
			respError: &JSONRPCError{
				Code:    1000,
				Message: "this is ignored",
			},
			expectedType:    &StaticError{},
			expectedMessage: "static error",
		},
		{
			name: "SimpleError",
			respError: &JSONRPCError{
				Code:    1001,
				Message: "simple error occurred",
			},
			expectedType:    &SimpleError{},
			expectedMessage: "simple error occurred",
		},
		{
			name: "DataStringError",
			respError: &JSONRPCError{
				Code:    1002,
				Message: "data error occurred",
				Data:    json.RawMessage(`"additional data"`),
			},
			expectedType:    &DataStringError{},
			expectedMessage: "data error occurred",
			verify: func(t *testing.T, err error) {
				require.Equal(t, "additional data", err.(*DataStringError).ErrorData())
			},
		},
		{
			name: "DataComplexError",
			respError: &JSONRPCError{
				Code:    1003,
				Message: "data error occurred",
				Data:    json.RawMessage(`{"foo":"boop","bar":101}`),
			},
			expectedType:    &DataComplexError{},
			expectedMessage: "data error occurred",
			verify: func(t *testing.T, err error) {
				require.Equal(t, ComplexData{Foo: "boop", Bar: 101}, err.(*DataComplexError).ErrorData())
			},
		},
		{
			name: "MetaError",
			respError: &JSONRPCError{
				Code:    1004,
				Message: "meta error occurred",
				Meta: func() json.RawMessage {
					me := &MetaError{
						Message: "meta error occurred",
						Details: "meta details",
					}
					metaData, _ := me.MarshalJSON()
					return metaData
				}(),
			},
			expectedType:    &MetaError{},
			expectedMessage: "meta error occurred",
			verify: func(t *testing.T, err error) {
				// details will also be included in the error message since it implements the marshable interface
				require.Equal(t, "meta details", err.(*MetaError).Details)
			},
		},
		{
			name: "ComplexError",
			respError: &JSONRPCError{
				Code:    1005,
				Message: "complex error occurred",
				Data:    json.RawMessage(`"complex data"`),
				Meta: func() json.RawMessage {
					ce := &ComplexError{
						Message: "complex error occurred",
						Details: "complex details",
						Data:    ComplexData{Foo: "foo", Bar: 42},
					}
					metaData, _ := ce.MarshalJSON()
					return metaData
				}(),
			},
			expectedType:    &ComplexError{},
			expectedMessage: "complex error occurred",
			verify: func(t *testing.T, err error) {
				require.Equal(t, ComplexData{Foo: "foo", Bar: 42}, err.(*ComplexError).ErrorData())
				require.Equal(t, "complex details", err.(*ComplexError).Details)
			},
		},
		{
			name: "UnregisteredError",
			respError: &JSONRPCError{
				Code:    9999,
				Message: "unregistered error occurred",
				Data:    json.RawMessage(`"some data"`),
			},
			expectedType:    &JSONRPCError{},
			expectedMessage: "unregistered error occurred",
			verify: func(t *testing.T, err error) {
				require.Equal(t, json.RawMessage(`"some data"`), err.(*JSONRPCError).ErrorData())
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			errValue := tc.respError.val(&errorsMap)
			errInterface := errValue.Interface()
			err, ok := errInterface.(error)
			require.True(t, ok, "returned value does not implement error interface")
			require.IsType(t, tc.expectedType, err)
			require.Equal(t, tc.expectedMessage, err.Error())
			if tc.verify != nil {
				tc.verify(t, err)
			}
		})
	}
}