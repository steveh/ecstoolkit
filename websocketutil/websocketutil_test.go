// Package websocketutil contains methods for interacting with websocket connections.
package websocketutil_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/websocketutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func handlerToBeTested(w http.ResponseWriter, req *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot upgrade: %v", err), http.StatusInternalServerError)
	}

	mt, p, err := conn.ReadMessage()
	if err != nil {
		return
	}

	if err := conn.WriteMessage(mt, []byte("hello "+string(p))); err != nil {
		log.NewMockLog().Error("Failed to write message", "error", err)
	}
}

func TestWebsocketUtilOpenCloseConnection(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(handlerToBeTested))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"

	log := log.NewMockLog()

	ws := websocketutil.NewWebsocketUtil(log, nil)
	conn, _ := ws.OpenConnection(u.String())
	assert.NotNil(t, conn, "Open connection failed.")

	err := ws.CloseConnection(conn)
	require.NoError(t, err, "Error closing the websocket connection.")
}

func TestWebsocketUtilOpenConnectionInvalidUrl(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(handlerToBeTested))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"

	log := log.NewMockLog()

	ws := websocketutil.NewWebsocketUtil(log, nil)
	conn, _ := ws.OpenConnection("InvalidUrl")
	assert.Nil(t, conn, "Open connection failed.")

	err := ws.CloseConnection(conn)
	require.Error(t, err, "Error closing the websocket connection.")
}

func TestSendMessage(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(handlerToBeTested))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"

	log := log.NewMockLog()

	ws := websocketutil.NewWebsocketUtil(log, nil)
	conn, _ := ws.OpenConnection(u.String())
	assert.NotNil(t, conn, "Open connection failed.")

	if err := conn.WriteMessage(websocket.TextMessage, []byte("testing")); err != nil {
		t.Errorf("Failed to write message: %v", err)
	}

	err := ws.CloseConnection(conn)
	require.NoError(t, err, "Error closing the websocket connection.")
}
