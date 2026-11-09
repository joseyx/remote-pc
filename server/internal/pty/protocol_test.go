package pty

import (
	"bytes"
	"testing"
)

func TestProtocolDataFraming(t *testing.T) {
	buf := new(bytes.Buffer)
	originalData := []byte("hello terminal \x1b[32mcolor\x1b[0m")

	if err := WriteDataFrame(buf, originalData); err != nil {
		t.Fatalf("WriteDataFrame failed: %v", err)
	}

	frameType, payload, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}

	if frameType != FrameData {
		t.Errorf("expected FrameData (0x00), got 0x%02x", frameType)
	}
	if !bytes.Equal(payload, originalData) {
		t.Errorf("expected payload %q, got %q", originalData, payload)
	}
}

func TestProtocolControlFraming(t *testing.T) {
	buf := new(bytes.Buffer)
	ctrl := &ControlMsg{
		Type:      "resize",
		SessionID: "sess-123",
		Cols:      140,
		Rows:      45,
		WorkerPID: 9999,
		ChildPID:  8888,
	}

	if err := WriteControlFrame(buf, ctrl); err != nil {
		t.Fatalf("WriteControlFrame failed: %v", err)
	}

	frameType, payload, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}

	if frameType != FrameControl {
		t.Errorf("expected FrameControl (0x01), got 0x%02x", frameType)
	}

	parsed, err := ParseControlFrame(payload)
	if err != nil {
		t.Fatalf("ParseControlFrame failed: %v", err)
	}

	if parsed.Type != ctrl.Type || parsed.Cols != ctrl.Cols || parsed.Rows != ctrl.Rows || parsed.ChildPID != ctrl.ChildPID {
		t.Errorf("mismatch in parsed control msg: %+v vs %+v", parsed, ctrl)
	}
}

func TestProtocolEmptyPayload(t *testing.T) {
	buf := new(bytes.Buffer)
	if err := WriteDataFrame(buf, []byte{}); err != nil {
		t.Fatalf("WriteDataFrame with empty slice failed: %v", err)
	}

	frameType, payload, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}

	if frameType != FrameData {
		t.Errorf("expected FrameData, got 0x%02x", frameType)
	}
	if len(payload) != 0 {
		t.Errorf("expected empty payload, got %d bytes", len(payload))
	}
}
