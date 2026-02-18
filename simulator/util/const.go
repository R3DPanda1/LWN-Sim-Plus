package util

// Constants
const (
	ConnectedPowerSource = 0

	MAXFCNTGAP = uint32(16384)
)

const (
	Stopped = iota
	Running

	Normal
	Retransmission
	FPending
	Activation
)
