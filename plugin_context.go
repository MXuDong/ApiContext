package ApiContext

import (
	"fmt"
	"reflect"
	"sync"
)

// NewContext return a new context instance
func NewContext() *ApiContext {
	ctx := &ApiContext{
		parentContext:     nil,
		childContexts:     []*ApiContext{},
		value:             map[interface{}]interface{}{},
		valueLock:         &sync.RWMutex{},
		listenerDoneCount: 0,
		listenerDoneChan:  make(chan struct{}),
		completeOnce:      &sync.Once{},
		contextLock:       &sync.Mutex{},
		completeFlag:      false,
	}

	return ctx
}

// ApiContext is the context of 'Plugins' and 'Core-Apis', for invoke quickly.
// ApiContext is not sync, because it will be invoked in one thread, if not, it needs to extend new context for invoke
// in child threads.
//
// TODO: Add cancel and dead for context support
type ApiContext struct {

	// The parentContext value, the parent type only the ApiContext
	parentContext *ApiContext
	// The childContexts list, the childContexts' type only the ApiContext
	childContexts []*ApiContext

	// the value map
	value map[interface{}]interface{}
	// the value lock, it can be provided outside
	valueLock *sync.RWMutex

	listenerDoneCount int
	listenerDoneChan  chan struct{}

	completeFlag bool
	completeOnce *sync.Once  // flag the complete, only can be invoked once
	contextLock  *sync.Mutex // the context's lock, for support thread-safe

	ApiError *ApiError
}

type ApiFunc func(a *ApiContext)

func (a *ApiContext) DoFuncWithName(name string, f ApiFunc) *ApiContext {
	context := a.QuickExtend()
	context.SetValue(KeyFuncName, name)
	context.SetValue(KeyFuncType, FuncBlockType)
	f(context)
	return context
}

// ConcurrentFuncWithName will invoke new func as sync.
func (a *ApiContext) ConcurrentFuncWithName(name string, f ApiFunc) *ApiContext {
	context := a.QuickExtend()
	context.SetValue(KeyFuncName, name)
	context.SetValue(KeyFuncType, FuncSyncType)
	go f(context)
	return context
}

// DoFunc will start invoke a new func
func (a *ApiContext) DoFunc(f ApiFunc) *ApiContext {
	return a.DoFuncWithName(defaultFuncName, f)
}

func (a *ApiContext) ConcurrentFunc(f ApiFunc) *ApiContext {
	return a.ConcurrentFuncWithName(defaultFuncName, f)
}

// Lock is context level lock, is so weight
func (a *ApiContext) Lock() {
	a.contextLock.Lock()
}

// Unlock the context
func (a *ApiContext) Unlock() {
	a.contextLock.Unlock()
}

// TODO: implement log-level output
func (a *ApiContext) Output(value string) {
	fmt.Println(value)
}

// AppendError will append an error to err-struck, but error should in one tree, different error can't in one struck.
// In other words, in different invoke struck, errors can't in one context.
func (a *ApiContext) AppendError(info string, object interface{}, errType ErrorType) *ApiError {
	a.contextLock.Lock()
	defer a.contextLock.Unlock()
	if a.ApiError == nil {
		a.ApiError = NewApiError(a.FuncName(), info, object, errType)
	} else {
		a.ApiError = a.ApiError.WithStruck(a.FuncName(), info, object, errType)
	}

	return a.ApiError
}

func (a *ApiContext) AppendErrorE(err error) {
	a.AppendError("", err, "Error")
}

// CatchLastError will return error and reset to last level error
func (a *ApiContext) CatchLastError() *ApiError {
	if a.ApiError == nil {
		return nil
	}
	a.contextLock.Lock()
	defer a.contextLock.Unlock()
	r := a.ApiError
	a.ApiError = a.ApiError.LastLevelError
	return r
}

// QuickExtend like Extend, but it hasn't to copy the value from parent.
func (a *ApiContext) QuickExtend() *ApiContext {
	childContext := NewContext()
	childContext.parentContext = a

	a.contextLock.Lock()
	a.childContexts = append(a.childContexts, childContext) // unsafe of thread
	a.contextLock.Unlock()

	return childContext
}

// Extend will copy all the value from parent.
func (a *ApiContext) Extend() *ApiContext {
	a.valueLock.Lock()
	a.contextLock.Lock()
	defer a.contextLock.Unlock()
	defer a.valueLock.Unlock()

	childContext := NewContext()
	childContext.parentContext = a

	a.childContexts = append(a.childContexts, childContext) // unsafe of thread

	// copy the value from a.value
	for k, v := range a.value {
		childContext.value[k] = v
	}

	return childContext
}

// Complete if be invoked, it can't provide any operator. And all of this context's children will be completed.
func (a *ApiContext) Complete() {
	a.contextLock.Lock()
	defer a.contextLock.Unlock()
	for _, childContext := range a.childContexts {
		childContext.Complete()
	}
	a.complete(struct{}{})
}

func (a *ApiContext) complete(param struct{}) {
	a.contextLock.Unlock()
	defer a.contextLock.Unlock()
	a.completeOnce.Do(func() {
		a.completeFlag = true
		for i := 0; i < a.listenerDoneCount; i++ {
			a.listenerDoneChan <- param
		}
	})
}

func (a *ApiContext) Done() <-chan struct{} {
	a.contextLock.Lock()
	defer a.contextLock.Unlock()
	a.listenerDoneCount++
	return a.listenerDoneChan
}

func (a *ApiContext) Err() error {
	return a.ApiError
}

// Contain return true when special key is exits
func (a *ApiContext) Contain(key interface{}) bool {
	_, ok := a.value[key]
	return ok
}

// CurrentValue will search value by key in current context.value
// CurrentValue is thread-safe.
func (a *ApiContext) CurrentValue(key interface{}) (interface{}, bool) {
	a.valueLock.RLock()
	defer a.valueLock.RUnlock()

	v, ok := a.value[key]
	return v, ok
}

// SetValue will set special key, if key already exits, overwrite it.
func (a *ApiContext) SetValue(key interface{}, v interface{}) {
	a.valueLock.Lock()
	defer a.valueLock.Unlock()

	a.value[key] = v
}

func (a *ApiContext) WithValue(key, v interface{}) *ApiContext {
	a.SetValue(key, v)
	return a
}

func (a *ApiContext) WithFuncName(name string) *ApiContext {
	a.SetValue(KeyFuncName, name)
	return a
}

func (a *ApiContext) WithFuncType(typ string) *ApiContext {
	a.SetValue(KeyFuncType, typ)
	return a
}

// ValueParent will search value by key in current context.value first, if not exits, and is deep, will search
// in parent context if it exits. But if it has not parents(the root context), will return def value.
// Inner of ValueParent, invoke the CurrentValue(key).
func (a *ApiContext) ValueParent(key interface{}, deep bool, def interface{}) interface{} {
	if value, ok := a.CurrentValue(key); ok {
		return value
	}
	if deep {
		if a.parentContext != nil {
			return a.parentContext.ValueParent(key, deep, def)
		}
	}

	return def
}

// Value will search use ValueParent(key, true, nil), it means, if current value not exits, it will search parent value.
func (a *ApiContext) Value(key interface{}) interface{} {
	return a.ValueParent(key, true, nil)
}

// ================================= for type funcs

// typeError return ApiError, only for build the type error
func typeError(funcName string, source interface{}, targetType string) *ApiError {
	typ := ""

	if source == nil {
		typ = "nil"
	} else {
		typ = reflect.TypeOf(typ).Name()
	}
	return NewApiError(funcName, fmt.Sprintf("'%s' can't convert to '%s'", typ, targetType), source, TypeError_ErrorType)
}

// StringValue return target key as value string, if value can't convert to the string
// return ""
func (a *ApiContext) StringValue(key interface{}) (string, *ApiError) {
	r := a.Value(key)
	if rString, ok := r.(string); ok {
		return rString, nil
	}
	return "", typeError("StringValue", r, reflect.String.String())
}

func (a *ApiContext) BoolValue(key interface{}) (bool, *ApiError) {
	r := a.Value(key)
	if rBool, ok := r.(bool); ok {
		return rBool, nil
	}
	return false, typeError("BoolValue", r, reflect.Bool.String())
}

func (a *ApiContext) IntValue(key interface{}) (int, *ApiError) {
	r := a.Value(key)
	if rInt, ok := r.(int); ok {
		return rInt, nil
	}
	return 0, typeError("IntValue", r, reflect.Int.String())
}

func (a *ApiContext) Float32Value(key interface{}) (float32, *ApiError) {
	r := a.Value(key)
	if rFloat32, ok := r.(float32); ok {
		return rFloat32, nil
	}
	return 0., typeError("Float32Value", r, reflect.Float32.String())
}

func (a *ApiContext) FuncName() string {
	funcName, err := a.StringValue(KeyFuncName)
	if err != nil {
		return UnknownFuncName
	}
	return funcName
}
