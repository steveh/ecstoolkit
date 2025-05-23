// Code generated by mockery v2.53.3. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// IEncrypter is an autogenerated mock type for the IEncrypter type
type IEncrypter struct {
	mock.Mock
}

type IEncrypter_Expecter struct {
	mock *mock.Mock
}

func (_m *IEncrypter) EXPECT() *IEncrypter_Expecter {
	return &IEncrypter_Expecter{mock: &_m.Mock}
}

// Decrypt provides a mock function with given fields: cipherText
func (_m *IEncrypter) Decrypt(cipherText []byte) ([]byte, error) {
	ret := _m.Called(cipherText)

	if len(ret) == 0 {
		panic("no return value specified for Decrypt")
	}

	var r0 []byte
	var r1 error
	if rf, ok := ret.Get(0).(func([]byte) ([]byte, error)); ok {
		return rf(cipherText)
	}
	if rf, ok := ret.Get(0).(func([]byte) []byte); ok {
		r0 = rf(cipherText)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	if rf, ok := ret.Get(1).(func([]byte) error); ok {
		r1 = rf(cipherText)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// IEncrypter_Decrypt_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Decrypt'
type IEncrypter_Decrypt_Call struct {
	*mock.Call
}

// Decrypt is a helper method to define mock.On call
//   - cipherText []byte
func (_e *IEncrypter_Expecter) Decrypt(cipherText interface{}) *IEncrypter_Decrypt_Call {
	return &IEncrypter_Decrypt_Call{Call: _e.mock.On("Decrypt", cipherText)}
}

func (_c *IEncrypter_Decrypt_Call) Run(run func(cipherText []byte)) *IEncrypter_Decrypt_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].([]byte))
	})
	return _c
}

func (_c *IEncrypter_Decrypt_Call) Return(plainText []byte, err error) *IEncrypter_Decrypt_Call {
	_c.Call.Return(plainText, err)
	return _c
}

func (_c *IEncrypter_Decrypt_Call) RunAndReturn(run func([]byte) ([]byte, error)) *IEncrypter_Decrypt_Call {
	_c.Call.Return(run)
	return _c
}

// Encrypt provides a mock function with given fields: plainText
func (_m *IEncrypter) Encrypt(plainText []byte) ([]byte, error) {
	ret := _m.Called(plainText)

	if len(ret) == 0 {
		panic("no return value specified for Encrypt")
	}

	var r0 []byte
	var r1 error
	if rf, ok := ret.Get(0).(func([]byte) ([]byte, error)); ok {
		return rf(plainText)
	}
	if rf, ok := ret.Get(0).(func([]byte) []byte); ok {
		r0 = rf(plainText)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	if rf, ok := ret.Get(1).(func([]byte) error); ok {
		r1 = rf(plainText)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// IEncrypter_Encrypt_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Encrypt'
type IEncrypter_Encrypt_Call struct {
	*mock.Call
}

// Encrypt is a helper method to define mock.On call
//   - plainText []byte
func (_e *IEncrypter_Expecter) Encrypt(plainText interface{}) *IEncrypter_Encrypt_Call {
	return &IEncrypter_Encrypt_Call{Call: _e.mock.On("Encrypt", plainText)}
}

func (_c *IEncrypter_Encrypt_Call) Run(run func(plainText []byte)) *IEncrypter_Encrypt_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].([]byte))
	})
	return _c
}

func (_c *IEncrypter_Encrypt_Call) Return(cipherText []byte, err error) *IEncrypter_Encrypt_Call {
	_c.Call.Return(cipherText, err)
	return _c
}

func (_c *IEncrypter_Encrypt_Call) RunAndReturn(run func([]byte) ([]byte, error)) *IEncrypter_Encrypt_Call {
	_c.Call.Return(run)
	return _c
}

// GetEncryptedDataKey provides a mock function with no fields
func (_m *IEncrypter) GetEncryptedDataKey() []byte {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetEncryptedDataKey")
	}

	var r0 []byte
	if rf, ok := ret.Get(0).(func() []byte); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	return r0
}

// IEncrypter_GetEncryptedDataKey_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetEncryptedDataKey'
type IEncrypter_GetEncryptedDataKey_Call struct {
	*mock.Call
}

// GetEncryptedDataKey is a helper method to define mock.On call
func (_e *IEncrypter_Expecter) GetEncryptedDataKey() *IEncrypter_GetEncryptedDataKey_Call {
	return &IEncrypter_GetEncryptedDataKey_Call{Call: _e.mock.On("GetEncryptedDataKey")}
}

func (_c *IEncrypter_GetEncryptedDataKey_Call) Run(run func()) *IEncrypter_GetEncryptedDataKey_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *IEncrypter_GetEncryptedDataKey_Call) Return(ciptherTextBlob []byte) *IEncrypter_GetEncryptedDataKey_Call {
	_c.Call.Return(ciptherTextBlob)
	return _c
}

func (_c *IEncrypter_GetEncryptedDataKey_Call) RunAndReturn(run func() []byte) *IEncrypter_GetEncryptedDataKey_Call {
	_c.Call.Return(run)
	return _c
}

// NewIEncrypter creates a new instance of IEncrypter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewIEncrypter(t interface {
	mock.TestingT
	Cleanup(func())
}) *IEncrypter {
	mock := &IEncrypter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
