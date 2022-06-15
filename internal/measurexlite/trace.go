package measurexlite

//
// Definition of Trace
//

import (
	"time"

	"github.com/ooni/probe-cli/v3/internal/model"
)

// Trace implements model.Trace.
//
// The zero-value of this struct is invalid. To construct you should either
// fill all the fields marked as MANDATORY or use NewTrace.
//
// NewTrace uses reasonable buffer sizes for the channels used for collecting
// events. You should drain the channels used by this implementation after
// each operation you perform (i.e., we expect you to peform step-by-step
// measurements). If you want larger (or smaller) buffers, then you should
// construct this data type manually with the desired buffer sizes.
//
// We have convenience methods for extracting events from the buffered
// channels. Otherwise, you could read the channels directly.
type Trace struct {
	// Index is the MANDATORY unique index of this trace within the current measurement.
	Index int64

	// NetworkEvent is MANDATORY and buffers network events.
	NetworkEvent chan *model.ArchivalNetworkEvent

	// TCPConnect is MANDATORY and buffers TCP connect observations.
	TCPConnect chan *model.ArchivalTCPConnectResult

	// TLSHandshake is MANDATORY and buffers TLS handshake observations.
	TLSHandshake chan *model.ArchivalTLSOrQUICHandshakeResult

	// ZeroTime is the MANDATORY time when we started the current measurement.
	ZeroTime time.Time
}

const (
	// NetworkEventBufferSize is the buffer size for constructing
	// the Trace's NetworkEvent buffered channel.
	NetworkEventBufferSize = 64

	// TCPConnectBufferSize is the buffer size for constructing
	// the Trace's TCPConnect buffered channel.
	TCPConnectBufferSize = 8

	// TLSHandshakeBufferSize is the buffer for construcing
	// the Trace's TLSHandshake buffered channel.
	TLSHandshakeBufferSize = 8
)

// NewTrace creates a new instance of Trace using default settings.
//
// Arguments:
//
// - index is the unique index of this trace within the current measurement;
//
// - zeroTime is the time when we started the current measurement.
func NewTrace(index int64, zeroTime time.Time) *Trace {
	return &Trace{
		Index:        index,
		NetworkEvent: make(chan *model.ArchivalNetworkEvent, NetworkEventBufferSize),
		TCPConnect:   make(chan *model.ArchivalTCPConnectResult, TCPConnectBufferSize),
		TLSHandshake: make(chan *model.ArchivalTLSOrQUICHandshakeResult, TLSHandshakeBufferSize),
		ZeroTime:     zeroTime,
	}
}

var _ model.Trace = &Trace{}