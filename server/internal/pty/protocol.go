package pty

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const (
	FrameData    byte = 0x00
	FrameControl byte = 0x01

	// MaxPayloadSize protects against corrupted packets (16 MB limit per frame)
	MaxPayloadSize = 16 * 1024 * 1024
)

// ControlMsg defines management messages exchanged between Orbit server and PTY worker
type ControlMsg struct {
	Type      string `json:"type"` // "hello", "resize", "ping", "pong", "exit", "close"
	SessionID string `json:"sessionId,omitempty"`
	Cols      uint16 `json:"cols,omitempty"`
	Rows      uint16 `json:"rows,omitempty"`
	WorkerPID int    `json:"workerPid,omitempty"`
	ChildPID  int    `json:"childPid,omitempty"`
	ExitCode  int    `json:"exitCode,omitempty"`
	Error     string `json:"error,omitempty"`
}

// WriteFrame writes a typed binary frame with a 5-byte header: [type: 1B][length: 4B][payload]
func WriteFrame(w io.Writer, frameType byte, payload []byte) error {
	payloadLen := len(payload)
	if payloadLen > MaxPayloadSize {
		return fmt.Errorf("payload size %d exceeds max frame size %d", payloadLen, MaxPayloadSize)
	}

	frame := make([]byte, 5+payloadLen)
	frame[0] = frameType
	binary.BigEndian.PutUint32(frame[1:5], uint32(payloadLen))
	if payloadLen > 0 {
		copy(frame[5:], payload)
	}

	_, err := w.Write(frame)
	return err
}

// ReadFrame reads a single frame from the stream
func ReadFrame(r io.Reader) (byte, []byte, error) {
	header := make([]byte, 5)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, nil, err
	}

	frameType := header[0]
	length := binary.BigEndian.Uint32(header[1:5])

	if length > MaxPayloadSize {
		return 0, nil, fmt.Errorf("frame size %d exceeds limit of %d", length, MaxPayloadSize)
	}

	if length == 0 {
		return frameType, []byte{}, nil
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}

	return frameType, payload, nil
}

// WriteDataFrame writes raw terminal bytes (stdin or stdout)
func WriteDataFrame(w io.Writer, data []byte) error {
	return WriteFrame(w, FrameData, data)
}

// WriteControlFrame encodes and writes a JSON control message
func WriteControlFrame(w io.Writer, msg *ControlMsg) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal control message: %w", err)
	}
	return WriteFrame(w, FrameControl, data)
}

// ParseControlFrame unmarshals a control frame payload
func ParseControlFrame(payload []byte) (*ControlMsg, error) {
	var msg ControlMsg
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal control message: %w", err)
	}
	return &msg, nil
}
