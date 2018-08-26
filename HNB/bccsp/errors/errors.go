
package errors

import (
	"bytes"
	"fmt"
	"regexp"
	"runtime"
)
const MaxCallStackLength = 30

var (
	componentPattern = "[A-Za-z]{3}"
	reasonPattern    = "[0-9]{3}"
)

type CallStackError interface {
	error
	GetStack() string
	GetErrorCode() string
	GetComponentCode() string
	GetReasonCode() string
	Message() string
	GenerateStack(bool) CallStackError
	WrapError(error) CallStackError
}

type callstack []uintptr

type callError struct {
	stack         callstack
	componentcode string
	reasoncode    string
	message       string
	args          []interface{}
	stackGetter   func(callstack) string
	prevErr       error
}

func setupCallError(e *callError, generateStack bool) {
	if !generateStack {
		e.stackGetter = noopGetStack
		return
	}
	e.stackGetter = getStack
	stack := make([]uintptr, MaxCallStackLength)
	skipCallersAndSetup := 2
	length := runtime.Callers(skipCallersAndSetup, stack[:])
	e.stack = stack[:length]
}

func (e *callError) Error() string {
	message := e.GetErrorCode() + " - " + fmt.Sprintf(e.message, e.args...)
	if e.GetStack() != "" {
		message = appendCallStack(message, e.GetStack())
	}
	if e.prevErr != nil {
		message += "\nCaused by: " + e.prevErr.Error()
	}
	return message
}

func (e *callError) GetStack() string {
	return e.stackGetter(e.stack)
}

func (e *callError) GetComponentCode() string {
	return e.componentcode
}

func (e *callError) GetReasonCode() string {
	return e.reasoncode
}

func (e *callError) GetErrorCode() string {
	return fmt.Sprintf("%s:%s", e.componentcode, e.reasoncode)
}

func (e *callError) Message() string {
	message := e.GetErrorCode() + " - " + fmt.Sprintf(e.message, e.args...)

	if e.prevErr != nil {
		switch previousError := e.prevErr.(type) {
		case CallStackError:
			message += "\nCaused by: " + previousError.Message()
		default:
			message += "\nCaused by: " + e.prevErr.Error()
		}
	}
	return message
}

func appendCallStack(message string, callstack string) string {
	return message + "\n" + callstack
}

func Error(componentcode string, reasoncode string, message string, args ...interface{}) CallStackError {
	return newError(componentcode, reasoncode, message, args...).GenerateStack(false)
}

func ErrorWithCallstack(componentcode string, reasoncode string, message string, args ...interface{}) CallStackError {
	return newError(componentcode, reasoncode, message, args...).GenerateStack(true)
}

func newError(componentcode string, reasoncode string, message string, args ...interface{}) CallStackError {
	e := &callError{}
	e.setErrorFields(componentcode, reasoncode, message, args...)
	return e
}

func (e *callError) GenerateStack(flag bool) CallStackError {
	setupCallError(e, flag)
	return e
}

func (e *callError) WrapError(prevErr error) CallStackError {
	e.prevErr = prevErr
	return e
}

func (e *callError) setErrorFields(componentcode string, reasoncode string, message string, args ...interface{}) {
	if isValidComponentOrReasonCode(componentcode, componentPattern) {
		e.componentcode = componentcode
	}
	if isValidComponentOrReasonCode(reasoncode, reasonPattern) {
		e.reasoncode = reasoncode
	}
	if message != "" {
		e.message = message
	}
	e.args = args
}

func isValidComponentOrReasonCode(componentOrReasonCode string, regExp string) bool {
	if componentOrReasonCode == "" {
		return false
	}
	re, _ := regexp.Compile(regExp)
	matched := re.FindString(componentOrReasonCode)
	if len(matched) != len(componentOrReasonCode) {
		return false
	}
	return true
}

func getStack(stack callstack) string {
	buf := bytes.Buffer{}
	if stack == nil {
		return fmt.Sprintf("No call stack available")
	}
	const firstNonErrorModuleCall int = 2
	stack = stack[firstNonErrorModuleCall:]
	for i, pc := range stack {
		f := runtime.FuncForPC(pc)
		file, line := f.FileLine(pc)
		if i != len(stack)-1 {
			buf.WriteString(fmt.Sprintf("%s:%d %s\n", file, line, f.Name()))
		} else {
			buf.WriteString(fmt.Sprintf("%s:%d %s", file, line, f.Name()))
		}
	}
	return fmt.Sprintf("%s", buf.Bytes())
}

func noopGetStack(stack callstack) string {
	return ""
}
