package test_example

import (
	"errors"
	assert "github.com/stretchr/testify/assert"
	"testing"
)

func TestServiceStart_FAILED(t *testing.T) {
	service := NewService()
	// After 2 second this start method will be failed and logger will give us a fatal
	// error as well as call our `Logger.ExitFunc` function. `Logger.ExitFunc` function gives us
	// a panic and we will recover this panic by the recover function.
	err := service.Start()
	expectedError := errors.New("cannot start service!")
	// Here check our expected error msg which comes from `Start()` method
  	if assert.Error(t, err) {
	   	assert.Equal(t, expectedError, err)
  	}
}