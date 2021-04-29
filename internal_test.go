package pinger

import (
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"
)

func TestTracker(t *testing.T) {
	tests := []struct {
		Bytes     []byte
		ProbeID   uint32
		MessageID uint32
		Error     error
	}{
		{[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 0x00000000, 0x00000000, nil},
		{[]byte{0x12, 0x34, 0x56, 0x78, 0x87, 0x65, 0x43, 0x21}, 0x12345678, 0x87654321, nil},
		{[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}, 0x01020304, 0x05060708, nil},
		{[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}, 0, 0, errInvalidTracker},
		{[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 0, 0, errInvalidTracker},
		{nil, 0, 0, errInvalidTracker},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%08X:%08X", tt.ProbeID, tt.MessageID), func(t *testing.T) {
			var tr tracker

			if err := (&tr).Unmarshal(tt.Bytes); err != tt.Error {
				t.Errorf("unexpected error on unmarshal: %#v != %#v", err.Error(), tt.Error.Error())
			}

			if tt.Error != nil {
				return
			}

			if tt.ProbeID != tr.ProbeID {
				t.Errorf("unexpected probe id: expected %08X but got %08X", tt.ProbeID, tr.ProbeID)
			}
			if tt.MessageID != tr.MessageID {
				t.Errorf("unexpected message id: expected %08X but got %08X", tt.MessageID, tr.MessageID)
			}

			tr.ProbeID = tt.ProbeID
			tr.MessageID = tt.MessageID

			bytes := tr.Marshal()
			if !reflect.DeepEqual(bytes, tt.Bytes) {
				t.Errorf("unexpected marshal bytes\nexpected: %v\n but got: %v", tt.Bytes, bytes)
			}
		})
	}
}

func TestResult(t *testing.T) {
	target, _ := net.ResolveIPAddr("ip", "127.0.0.1")

	r := newResult(target, 4)

	r.onRecv(1 * time.Second)
	r.onRecv(2 * time.Second)
	r.onRecv(6 * time.Second)

	r.calculate()

	if r.Sent != 0 {
		t.Errorf("unexpected sent packets: %d", r.Sent)
	}
	if r.Recv != 3 {
		t.Errorf("unexpected received packets: %d", r.Recv)
	}
	if r.Loss != 1 {
		t.Errorf("unexpected lose packets: %d", r.Loss)
	}

	if r.MinRTT != 1*time.Second {
		t.Errorf("unexpected minimal RTT: %s", r.MinRTT)
	}
	if r.MaxRTT != 6*time.Second {
		t.Errorf("unexpected maximum RTT: %s", r.MaxRTT)
	}
	if r.AvgRTT != 3*time.Second {
		t.Errorf("unexpected average RTT: %s", r.AvgRTT)
	}
}
