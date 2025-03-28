// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package websocketutil contains methods for interacting with websocket connections.
package websocketutil

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/steveh/ecstoolkit/log"
	"github.com/stretchr/testify/assert"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func handlerToBeTested(w http.ResponseWriter, req *http.Request) {
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot upgrade: %v", err), http.StatusInternalServerError)
	}

	mt, p, err := conn.ReadMessage()
	if err != nil {
		return
	}

	conn.WriteMessage(mt, []byte("hello "+string(p)))
}

func TestWebsocketUtilOpenCloseConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(handlerToBeTested))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"

	log := log.NewMockLog()

	ws := NewWebsocketUtil(log, nil)
	conn, _ := ws.OpenConnection(u.String())
	assert.NotNil(t, conn, "Open connection failed.")

	err := ws.CloseConnection(conn)
	assert.NoError(t, err, "Error closing the websocket connection.")
}

func TestWebsocketUtilOpenConnectionInvalidUrl(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(handlerToBeTested))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"

	log := log.NewMockLog()

	ws := NewWebsocketUtil(log, nil)
	conn, _ := ws.OpenConnection("InvalidUrl")
	assert.Nil(t, conn, "Open connection failed.")

	err := ws.CloseConnection(conn)
	assert.Error(t, err, "Error closing the websocket connection.")
}

func TestSendMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(handlerToBeTested))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"

	log := log.NewMockLog()

	ws := NewWebsocketUtil(log, nil)
	conn, _ := ws.OpenConnection(u.String())
	assert.NotNil(t, conn, "Open connection failed.")
	conn.WriteMessage(websocket.TextMessage, []byte("testing"))

	err := ws.CloseConnection(conn)
	assert.NoError(t, err, "Error closing the websocket connection.")
}
