// Code generated by mockery v2.53.3. DO NOT EDIT.

package mocks

import (
	log "github.com/steveh/ecstoolkit/log"
	mock "github.com/stretchr/testify/mock"

	time "time"
)

// IWebSocketChannel is an autogenerated mock type for the IWebSocketChannel type
type IWebSocketChannel struct {
	mock.Mock
}

type IWebSocketChannel_Expecter struct {
	mock *mock.Mock
}

func (_m *IWebSocketChannel) EXPECT() *IWebSocketChannel_Expecter {
	return &IWebSocketChannel_Expecter{mock: &_m.Mock}
}

// Close provides a mock function with given fields: _a0
func (_m *IWebSocketChannel) Close(_a0 log.T) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for Close")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(log.T) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// IWebSocketChannel_Close_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Close'
type IWebSocketChannel_Close_Call struct {
	*mock.Call
}

// Close is a helper method to define mock.On call
//   - _a0 log.T
func (_e *IWebSocketChannel_Expecter) Close(_a0 interface{}) *IWebSocketChannel_Close_Call {
	return &IWebSocketChannel_Close_Call{Call: _e.mock.On("Close", _a0)}
}

func (_c *IWebSocketChannel_Close_Call) Run(run func(_a0 log.T)) *IWebSocketChannel_Close_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(log.T))
	})
	return _c
}

func (_c *IWebSocketChannel_Close_Call) Return(_a0 error) *IWebSocketChannel_Close_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *IWebSocketChannel_Close_Call) RunAndReturn(run func(log.T) error) *IWebSocketChannel_Close_Call {
	_c.Call.Return(run)
	return _c
}

// GetChannelToken provides a mock function with no fields
func (_m *IWebSocketChannel) GetChannelToken() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetChannelToken")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// IWebSocketChannel_GetChannelToken_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetChannelToken'
type IWebSocketChannel_GetChannelToken_Call struct {
	*mock.Call
}

// GetChannelToken is a helper method to define mock.On call
func (_e *IWebSocketChannel_Expecter) GetChannelToken() *IWebSocketChannel_GetChannelToken_Call {
	return &IWebSocketChannel_GetChannelToken_Call{Call: _e.mock.On("GetChannelToken")}
}

func (_c *IWebSocketChannel_GetChannelToken_Call) Run(run func()) *IWebSocketChannel_GetChannelToken_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *IWebSocketChannel_GetChannelToken_Call) Return(_a0 string) *IWebSocketChannel_GetChannelToken_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *IWebSocketChannel_GetChannelToken_Call) RunAndReturn(run func() string) *IWebSocketChannel_GetChannelToken_Call {
	_c.Call.Return(run)
	return _c
}

// GetStreamUrl provides a mock function with no fields
func (_m *IWebSocketChannel) GetStreamUrl() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetStreamUrl")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// IWebSocketChannel_GetStreamUrl_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetStreamUrl'
type IWebSocketChannel_GetStreamUrl_Call struct {
	*mock.Call
}

// GetStreamUrl is a helper method to define mock.On call
func (_e *IWebSocketChannel_Expecter) GetStreamUrl() *IWebSocketChannel_GetStreamUrl_Call {
	return &IWebSocketChannel_GetStreamUrl_Call{Call: _e.mock.On("GetStreamUrl")}
}

func (_c *IWebSocketChannel_GetStreamUrl_Call) Run(run func()) *IWebSocketChannel_GetStreamUrl_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *IWebSocketChannel_GetStreamUrl_Call) Return(_a0 string) *IWebSocketChannel_GetStreamUrl_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *IWebSocketChannel_GetStreamUrl_Call) RunAndReturn(run func() string) *IWebSocketChannel_GetStreamUrl_Call {
	_c.Call.Return(run)
	return _c
}

// Initialize provides a mock function with given fields: _a0, channelUrl, channelToken
func (_m *IWebSocketChannel) Initialize(_a0 log.T, channelUrl string, channelToken string) {
	_m.Called(_a0, channelUrl, channelToken)
}

// IWebSocketChannel_Initialize_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Initialize'
type IWebSocketChannel_Initialize_Call struct {
	*mock.Call
}

// Initialize is a helper method to define mock.On call
//   - _a0 log.T
//   - channelUrl string
//   - channelToken string
func (_e *IWebSocketChannel_Expecter) Initialize(_a0 interface{}, channelUrl interface{}, channelToken interface{}) *IWebSocketChannel_Initialize_Call {
	return &IWebSocketChannel_Initialize_Call{Call: _e.mock.On("Initialize", _a0, channelUrl, channelToken)}
}

func (_c *IWebSocketChannel_Initialize_Call) Run(run func(_a0 log.T, channelUrl string, channelToken string)) *IWebSocketChannel_Initialize_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(log.T), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *IWebSocketChannel_Initialize_Call) Return() *IWebSocketChannel_Initialize_Call {
	_c.Call.Return()
	return _c
}

func (_c *IWebSocketChannel_Initialize_Call) RunAndReturn(run func(log.T, string, string)) *IWebSocketChannel_Initialize_Call {
	_c.Run(run)
	return _c
}

// Open provides a mock function with given fields: _a0
func (_m *IWebSocketChannel) Open(_a0 log.T) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for Open")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(log.T) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// IWebSocketChannel_Open_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Open'
type IWebSocketChannel_Open_Call struct {
	*mock.Call
}

// Open is a helper method to define mock.On call
//   - _a0 log.T
func (_e *IWebSocketChannel_Expecter) Open(_a0 interface{}) *IWebSocketChannel_Open_Call {
	return &IWebSocketChannel_Open_Call{Call: _e.mock.On("Open", _a0)}
}

func (_c *IWebSocketChannel_Open_Call) Run(run func(_a0 log.T)) *IWebSocketChannel_Open_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(log.T))
	})
	return _c
}

func (_c *IWebSocketChannel_Open_Call) Return(_a0 error) *IWebSocketChannel_Open_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *IWebSocketChannel_Open_Call) RunAndReturn(run func(log.T) error) *IWebSocketChannel_Open_Call {
	_c.Call.Return(run)
	return _c
}

// SendMessage provides a mock function with given fields: _a0, input, inputType
func (_m *IWebSocketChannel) SendMessage(_a0 log.T, input []byte, inputType int) error {
	ret := _m.Called(_a0, input, inputType)

	if len(ret) == 0 {
		panic("no return value specified for SendMessage")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(log.T, []byte, int) error); ok {
		r0 = rf(_a0, input, inputType)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// IWebSocketChannel_SendMessage_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SendMessage'
type IWebSocketChannel_SendMessage_Call struct {
	*mock.Call
}

// SendMessage is a helper method to define mock.On call
//   - _a0 log.T
//   - input []byte
//   - inputType int
func (_e *IWebSocketChannel_Expecter) SendMessage(_a0 interface{}, input interface{}, inputType interface{}) *IWebSocketChannel_SendMessage_Call {
	return &IWebSocketChannel_SendMessage_Call{Call: _e.mock.On("SendMessage", _a0, input, inputType)}
}

func (_c *IWebSocketChannel_SendMessage_Call) Run(run func(_a0 log.T, input []byte, inputType int)) *IWebSocketChannel_SendMessage_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(log.T), args[1].([]byte), args[2].(int))
	})
	return _c
}

func (_c *IWebSocketChannel_SendMessage_Call) Return(_a0 error) *IWebSocketChannel_SendMessage_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *IWebSocketChannel_SendMessage_Call) RunAndReturn(run func(log.T, []byte, int) error) *IWebSocketChannel_SendMessage_Call {
	_c.Call.Return(run)
	return _c
}

// SetChannelToken provides a mock function with given fields: _a0
func (_m *IWebSocketChannel) SetChannelToken(_a0 string) {
	_m.Called(_a0)
}

// IWebSocketChannel_SetChannelToken_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SetChannelToken'
type IWebSocketChannel_SetChannelToken_Call struct {
	*mock.Call
}

// SetChannelToken is a helper method to define mock.On call
//   - _a0 string
func (_e *IWebSocketChannel_Expecter) SetChannelToken(_a0 interface{}) *IWebSocketChannel_SetChannelToken_Call {
	return &IWebSocketChannel_SetChannelToken_Call{Call: _e.mock.On("SetChannelToken", _a0)}
}

func (_c *IWebSocketChannel_SetChannelToken_Call) Run(run func(_a0 string)) *IWebSocketChannel_SetChannelToken_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *IWebSocketChannel_SetChannelToken_Call) Return() *IWebSocketChannel_SetChannelToken_Call {
	_c.Call.Return()
	return _c
}

func (_c *IWebSocketChannel_SetChannelToken_Call) RunAndReturn(run func(string)) *IWebSocketChannel_SetChannelToken_Call {
	_c.Run(run)
	return _c
}

// SetOnError provides a mock function with given fields: onErrorHandler
func (_m *IWebSocketChannel) SetOnError(onErrorHandler func(error)) {
	_m.Called(onErrorHandler)
}

// IWebSocketChannel_SetOnError_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SetOnError'
type IWebSocketChannel_SetOnError_Call struct {
	*mock.Call
}

// SetOnError is a helper method to define mock.On call
//   - onErrorHandler func(error)
func (_e *IWebSocketChannel_Expecter) SetOnError(onErrorHandler interface{}) *IWebSocketChannel_SetOnError_Call {
	return &IWebSocketChannel_SetOnError_Call{Call: _e.mock.On("SetOnError", onErrorHandler)}
}

func (_c *IWebSocketChannel_SetOnError_Call) Run(run func(onErrorHandler func(error))) *IWebSocketChannel_SetOnError_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(func(error)))
	})
	return _c
}

func (_c *IWebSocketChannel_SetOnError_Call) Return() *IWebSocketChannel_SetOnError_Call {
	_c.Call.Return()
	return _c
}

func (_c *IWebSocketChannel_SetOnError_Call) RunAndReturn(run func(func(error))) *IWebSocketChannel_SetOnError_Call {
	_c.Run(run)
	return _c
}

// SetOnMessage provides a mock function with given fields: onMessageHandler
func (_m *IWebSocketChannel) SetOnMessage(onMessageHandler func([]byte)) {
	_m.Called(onMessageHandler)
}

// IWebSocketChannel_SetOnMessage_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SetOnMessage'
type IWebSocketChannel_SetOnMessage_Call struct {
	*mock.Call
}

// SetOnMessage is a helper method to define mock.On call
//   - onMessageHandler func([]byte)
func (_e *IWebSocketChannel_Expecter) SetOnMessage(onMessageHandler interface{}) *IWebSocketChannel_SetOnMessage_Call {
	return &IWebSocketChannel_SetOnMessage_Call{Call: _e.mock.On("SetOnMessage", onMessageHandler)}
}

func (_c *IWebSocketChannel_SetOnMessage_Call) Run(run func(onMessageHandler func([]byte))) *IWebSocketChannel_SetOnMessage_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(func([]byte)))
	})
	return _c
}

func (_c *IWebSocketChannel_SetOnMessage_Call) Return() *IWebSocketChannel_SetOnMessage_Call {
	_c.Call.Return()
	return _c
}

func (_c *IWebSocketChannel_SetOnMessage_Call) RunAndReturn(run func(func([]byte))) *IWebSocketChannel_SetOnMessage_Call {
	_c.Run(run)
	return _c
}

// StartPings provides a mock function with given fields: _a0, pingInterval
func (_m *IWebSocketChannel) StartPings(_a0 log.T, pingInterval time.Duration) {
	_m.Called(_a0, pingInterval)
}

// IWebSocketChannel_StartPings_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'StartPings'
type IWebSocketChannel_StartPings_Call struct {
	*mock.Call
}

// StartPings is a helper method to define mock.On call
//   - _a0 log.T
//   - pingInterval time.Duration
func (_e *IWebSocketChannel_Expecter) StartPings(_a0 interface{}, pingInterval interface{}) *IWebSocketChannel_StartPings_Call {
	return &IWebSocketChannel_StartPings_Call{Call: _e.mock.On("StartPings", _a0, pingInterval)}
}

func (_c *IWebSocketChannel_StartPings_Call) Run(run func(_a0 log.T, pingInterval time.Duration)) *IWebSocketChannel_StartPings_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(log.T), args[1].(time.Duration))
	})
	return _c
}

func (_c *IWebSocketChannel_StartPings_Call) Return() *IWebSocketChannel_StartPings_Call {
	_c.Call.Return()
	return _c
}

func (_c *IWebSocketChannel_StartPings_Call) RunAndReturn(run func(log.T, time.Duration)) *IWebSocketChannel_StartPings_Call {
	_c.Run(run)
	return _c
}

// NewIWebSocketChannel creates a new instance of IWebSocketChannel. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewIWebSocketChannel(t interface {
	mock.TestingT
	Cleanup(func())
}) *IWebSocketChannel {
	mock := &IWebSocketChannel{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
