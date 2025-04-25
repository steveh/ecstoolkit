package message

import "github.com/steveh/ecstoolkit/log"

// IClientMessage defines the interface for client message operations.
type IClientMessage interface {
	Validate() error
	DeserializeClientMessage(log log.T, input []byte) (err error)
	SerializeClientMessage(log log.T) (result []byte, err error)
	DeserializeDataStreamAcknowledgeContent(log log.T) (dataStreamAcknowledge AcknowledgeContent, err error)
	DeserializeChannelClosedMessage(log log.T) (channelClosed ChannelClosed, err error)
	DeserializeHandshakeRequest(log log.T) (handshakeRequest HandshakeRequestPayload, err error)
	DeserializeHandshakeComplete(log log.T) (handshakeComplete HandshakeCompletePayload, err error)
}
