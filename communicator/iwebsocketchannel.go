package communicator

// IWebSocketChannel is the interface for DataChannel.
type IWebSocketChannel interface {
	Open() error
	Close() error
	ReadMessage() ([]byte, error)
	SendMessage(input []byte, inputType int) error
	GetChannelToken() string
	GetStreamURL() string
	SetChannelToken(channelToken string)
}
