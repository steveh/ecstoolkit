// Package websocketutil contains methods for interacting with websocket connections.
package websocketutil

import (
	"errors"
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/steveh/ecstoolkit/log"
)

// ErrNil is returned when the websocket connection is nil.
var ErrNil = errors.New("websocket is nil")

// IWebsocketUtil is the interface for the websocketutil.
type IWebsocketUtil interface {
	OpenConnection(url string) (*websocket.Conn, error)
	CloseConnection(ws websocket.Conn) error
}

// WebsocketUtil struct provides functionality around creating and maintaining websockets.
type WebsocketUtil struct {
	dialer *websocket.Dialer
	log    log.T
}

// NewWebsocketUtil is the factory function for websocketutil.
func NewWebsocketUtil(logger log.T, dialerInput *websocket.Dialer) *WebsocketUtil {
	var websocketUtil *WebsocketUtil

	if dialerInput == nil {
		websocketUtil = &WebsocketUtil{
			dialer: websocket.DefaultDialer,
			log:    logger,
		}
	} else {
		websocketUtil = &WebsocketUtil{
			dialer: dialerInput,
			log:    logger,
		}
	}

	return websocketUtil
}

// OpenConnection opens a websocket connection provided an input url.
func (u *WebsocketUtil) OpenConnection(url string) (*websocket.Conn, error) {
	u.log.Debug("Opening websocket connection", "url", url)

	conn, _, err := u.dialer.Dial(url, nil)
	if err != nil {
		u.log.Error("dialing websocket", "error", err.Error())

		return nil, fmt.Errorf("dialing websocket: %w", err)
	}

	u.log.Debug("Websocket connection opened")

	return conn, nil
}

// CloseConnection closes a websocket connection given the Conn object as input.
func (u *WebsocketUtil) CloseConnection(ws *websocket.Conn) error {
	if ws == nil {
		return ErrNil
	}

	u.log.Debug("Closing websocket connection", "remoteAddr", ws.RemoteAddr().String())

	err := ws.Close()
	if err != nil {
		u.log.Error("closing websocket", "error", err.Error())

		return fmt.Errorf("closing websocket: %w", err)
	}

	u.log.Debug("Successfully closed websocket connection", "remoteAddr", ws.RemoteAddr().String())

	return nil
}
