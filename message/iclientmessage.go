package message

// IClientMessage defines the interface for client message operations.
type IClientMessage interface {
	Validate() error
	DeserializeClientMessage(input []byte) (err error)
	SerializeClientMessage() (result []byte, err error)
	DeserializeDataStreamAcknowledgeContent() (dataStreamAcknowledge AcknowledgeContent, err error)
	DeserializeChannelClosedMessage() (channelClosed ChannelClosed, err error)
	DeserializeHandshakeRequest() (handshakeRequest HandshakeRequestPayload, err error)
	DeserializeHandshakeComplete() (handshakeComplete HandshakeCompletePayload, err error)
}
