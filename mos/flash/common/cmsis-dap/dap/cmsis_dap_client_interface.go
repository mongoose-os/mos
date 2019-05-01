package dap

import (
	"bytes"
	"context"
	"time"
)

type DAPClient interface {
	GetInfo(ctx context.Context, info uint8) (*bytes.Buffer, error)
	GetVendorID(ctx context.Context) (string, error)
	GetProductID(ctx context.Context) (string, error)
	GetSerialNumber(ctx context.Context) (string, error)
	GetFirmwareVersion(ctx context.Context) (string, error)
	GetTargetVendor(ctx context.Context) (string, error)
	GetTargetName(ctx context.Context) (string, error)

	SetHostStatus(ctx context.Context, st StatusType, value bool) error
	Connect(ctx context.Context, mode ConnectMode) error
	Disconnect(ctx context.Context) error
	TransferConfigure(ctx context.Context, idleCycles uint8, waitRetry uint16, matchRetry uint16) error
	Transfer(ctx context.Context, dapIndex uint8, reqs []TransferRequest) (TransferStatus, []uint32, error)
	GetTransferBlockMaxSize() int
	TransferBlockRead(ctx context.Context, dapIndex uint8, ap bool, reg uint8, length int) ([]uint32, error)
	TransferBlockWrite(ctx context.Context, dapIndex uint8, ap bool, reg uint8, data []uint32) error
	Delay(ctx context.Context, delay time.Duration) error
	ResetTarget(ctx context.Context) error
	SWJClock(ctx context.Context, clockHz uint32) error
	SWJSequence(ctx context.Context, numBits int, data []uint8) error
	SWDConfigure(ctx context.Context, config uint8) error

	Close(ctx context.Context) error
}

type StatusType uint8

const (
	StatusConnected StatusType = 0x00
	StatusRunning              = 0x01
)

type ConnectMode uint8

const (
	ConnectModeAuto ConnectMode = 0x00
	ConnectModeSWD              = 0x01
	ConnectModeJTAG             = 0x02
)

type TransferOp uint8

const (
	OpRead       TransferOp = 0
	OpReadMatch             = 1
	OpWrite                 = 2
	OpWriteMatch            = 3
)

type TransferRequest struct {
	Op   TransferOp
	AP   bool
	Reg  uint8
	Data uint32
}

type TransferStatus uint8

const (
	TransferStatusWait TransferStatus = 2
)

func (ts TransferStatus) Ok() bool {
	return ts.AckValue() == 1 && !ts.SWDError() && !ts.ValueMismatch()
}

func (ts TransferStatus) AckValue() uint8 {
	return uint8(ts & 7)
}

func (ts TransferStatus) SWDError() bool {
	return ts&8 != 0
}

func (ts TransferStatus) ValueMismatch() bool {
	return ts&0x10 != 0
}
