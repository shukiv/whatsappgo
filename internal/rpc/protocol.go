package rpc

import "encoding/json"

const ProtocolVersion = 1

type Request struct {
	Version int             `json:"version"`
	ID      string          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	Version int    `json:"version"`
	ID      string `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type Event struct {
	Version int    `json:"version"`
	Event   string `json:"event"`
	Data    any    `json:"data,omitempty"`
}
