package callback

import (
	"fmt"
	"strings"
)

var (
	// ErrNotSupport is returned when the callback handler does not support
	// the callback format.
	ErrNotSupport = fmt.Errorf("no support")
)

var callbacks = map[string]Callback{}

// Callback defines an interface of the callback notification handler.
type Callback interface {
	// Callback is the handler to parse the callback address and send
	// the notification.
	//
	// The second argument, value, is the changed and new value.
	//
	// If the implementation cannot parse or handle the callback, it should
	// return an error, ErrNotSupport.
	Callback(callback string, value string) error
}

// CallbackFunc converts a function to Callback.
type CallbackFunc func(callback string, value string) error

// Callback implements the interface Callback.
func (c CallbackFunc) Callback(callback, value string) error {
	return c(callback, value)
}

// RegisterCallback registers a callback notification handler with a name.
//
// Notice: If the name has been registered, it will replace the old one and
// return it. If callback is nil, it will panic.
func RegisterCallback(name string, callback Callback) (old Callback) {
	if callback == nil {
		panic("the callback is nil")
	}

	old = callbacks[name]
	callbacks[name] = callback
	return
}

// Notify calls the callback handlers in turn to notify the app that the value
// has been changed.
func Notify(in <-chan map[string][2]string, out chan<- map[string][2]string) {
	for {
		cbs := <-in
		go handleCallback(out, cbs)
	}
}

func handleCallback(out chan<- map[string][2]string, cbs map[string][2]string) {
	results := make(map[string][2]string, len(cbs))
	for id, cb := range cbs {
		var result string
		func() {
			defer func() {
				if err := recover(); err != nil {
					result = fmt.Sprintf("callback panic: %v", err)
				}
			}()

			errs := make([]string, 0, len(callbacks))
			for name, callback := range callbacks {
				err := callback.Callback(cb[0], cb[1])
				if err == nil {
					result = ""
					return
				}
				errs = append(errs, fmt.Sprintf("cb[%s]: %s", name, err))
			}
			result = strings.Join(errs, ";; ")
		}()
		results[id] = [2]string{cb[0], result}
	}
	out <- results
}
