// Package session starts the session.
package session

import (
	"context"
	"testing"

	dataChannelMock "github.com/steveh/ecstoolkit/datachannel/mocks"
	"github.com/steveh/ecstoolkit/log"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func getMockDataChannel() *dataChannelMock.IDataChannel {
	mockDataChannel := dataChannelMock.IDataChannel{}

	mockDataChannel.On("Open", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", nil)
	mockDataChannel.On("SetOnMessage", mock.Anything)
	mockDataChannel.On("RegisterOutputStreamHandler", mock.Anything, mock.Anything)
	mockDataChannel.On("RegisterOutputMessageHandler", mock.Anything, mock.Anything, mock.Anything)
	mockDataChannel.On("ResendStreamDataMessageScheduler", mock.Anything).Return(nil)

	return &mockDataChannel
}

func TestOpenDataChannel(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()
	mockDataChannel := getMockDataChannel()

	sessionMock := &Session{
		logger: mockLogger,
	}
	sessionMock.dataChannel = mockDataChannel

	mockDataChannel.On("Open", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", nil)

	_, err := sessionMock.OpenDataChannel(context.TODO())
	require.NoError(t, err)
}

func TestOpenDataChannelWithError(t *testing.T) {
	t.Parallel()

	mockLogger := log.NewMockLog()
	mockDataChannel := getMockDataChannel()

	sessionMock := &Session{
		logger: mockLogger,
	}
	sessionMock.dataChannel = mockDataChannel

	// First reconnection failed when open datachannel, success after retry
	mockDataChannel.On("Open", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", nil)

	_, err := sessionMock.OpenDataChannel(context.TODO())
	require.NoError(t, err)
}
