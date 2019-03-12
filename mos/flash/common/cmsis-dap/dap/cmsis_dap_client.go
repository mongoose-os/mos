// +build !no_libudev

package dap

// This package implements (a subset of) the CMSIS-DAP probe interface
// https://arm-software.github.io/CMSIS_5/DAP/html/group__DAP__Commands__gr.html

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"time"

	"github.com/cesanta/errors"
	"github.com/cesanta/hid"
	"github.com/golang/glog"
)

type cmd uint8

const (
	cmdInfo              cmd = 0x00
	cmdSetHostStatus         = 0x01
	cmdConnect               = 0x02
	cmdDisconnect            = 0x03
	cmdTransferConfigure     = 0x04
	cmdTransfer              = 0x05
	cmdTransferBlock         = 0x06
	cmdDelay                 = 0x09
	cmdResetTarget           = 0x0a
	cmdSWJClock              = 0x11
	cmdSWJSequence           = 0x12
	cmdSWDConfigure          = 0x13
)

type dapClient struct {
	d             hid.Device
	di            *hid.DeviceInfo
	maxPacketSize int
}

func NewClient(ctx context.Context, vid, pid uint16, serial string, intf, epIn, epOut int) (DAPClient, error) {
	devs, err := hid.Devices()
	if err != nil {
		return nil, errors.Annotatef(err, "failed to enumerate HID devices")
	}
	for i, di := range devs {
		glog.V(1).Infof("%d: %04x:%04x %s", i, di.VendorID, di.ProductID, di.Path)
		// TODO(rojer): Serial number matching
		if di.VendorID == vid && di.ProductID == pid {
			d, err := di.Open()
			if err != nil {
				return nil, errors.Annotatef(err, "failed to open device %04x:%04x (%s)", di.VendorID, di.ProductID, di.Path)
			}
			glog.Infof("Opened %04x:%04x (%s)", di.VendorID, di.ProductID, di.Path)
			dapc := &dapClient{
				di:            di,
				d:             d,
				maxPacketSize: 8, // Start with a conservative guess
			}
			resp, err := dapc.GetInfo(ctx, 0xff)
			if err != nil {
				dapc.Close(ctx)
				return nil, errors.Annotatef(err, "failed to get max packet size")
			}
			var rl uint8
			var mps uint16
			binary.Read(resp, binary.LittleEndian, &rl)
			binary.Read(resp, binary.LittleEndian, &mps)
			dapc.maxPacketSize = int(mps)
			glog.V(2).Infof("max packet size: %d", dapc.maxPacketSize)
			return dapc, nil
		}
	}
	return nil, errors.NotFoundf("device %04x:%04x", vid, pid)
}

func newCmd(cmd cmd) *bytes.Buffer {
	return bytes.NewBuffer([]uint8{
		0, // HID report number (unused)
		uint8(cmd),
	})
}

func (dapc *dapClient) exec(ctx context.Context, args *bytes.Buffer) (*bytes.Buffer, error) {
	glog.V(4).Infof(" => %s", hex.EncodeToString(args.Bytes()[1:]))
	if len(args.Bytes()) > dapc.maxPacketSize {
		return nil, errors.Errorf("packet too long (max %d, got %d)", dapc.maxPacketSize, len(args.Bytes()))
	}
	if err := dapc.d.Write(args.Bytes()); err != nil {
		return nil, errors.Annotatef(err, "device write failed", err)
	}
	select {
	case <-ctx.Done():
		return nil, errors.Annotatef(ctx.Err(), "DAP exec")
	case resp, ok := <-dapc.d.ReadCh():
		if !ok {
			return nil, errors.Annotatef(dapc.d.ReadError(), "device read failed")
		}
		glog.V(4).Infof("<=  %s", hex.EncodeToString(resp))
		cmd := args.Bytes()[1]
		if resp[0] != cmd {
			return nil, errors.Errorf("Response to wrong command (want 0x%02x, got 0x%02x)", cmd, resp[0])
		}
		return bytes.NewBuffer(resp[1:]), nil
	}
}

func (dapc *dapClient) execCheckStatus(ctx context.Context, args *bytes.Buffer) error {
	resp, err := dapc.exec(ctx, args)
	cmd := args.Bytes()[1]
	status := resp.Bytes()[0]
	if status != 0 {
		return errors.Errorf("Command 0x%02x returned error (0x%02x)", cmd, status)
	}
	return err
}

func (dapc *dapClient) GetInfo(ctx context.Context, info uint8) (*bytes.Buffer, error) {
	glog.V(3).Infof("GetInfo(%d)", info)
	args := newCmd(cmdInfo)
	binary.Write(args, binary.LittleEndian, info)
	resp, err := dapc.exec(ctx, args)
	return resp, errors.Annotatef(err, "failed to get info 0x%02x", info)
}

func (dapc *dapClient) GetInfoString(ctx context.Context, info uint8) (string, error) {
	resp, err := dapc.GetInfo(ctx, info)
	if err != nil {
		return "", errors.Trace(err)
	}
	var sl uint8
	binary.Read(resp, binary.LittleEndian, &sl)
	s := make([]uint8, sl)
	resp.Read(s)
	return string(s), nil
}

func (dapc *dapClient) GetVendorID(ctx context.Context) (string, error) {
	return dapc.GetInfoString(ctx, 1)
}

func (dapc *dapClient) GetProductID(ctx context.Context) (string, error) {
	return dapc.GetInfoString(ctx, 2)
}

func (dapc *dapClient) GetSerialNumber(ctx context.Context) (string, error) {
	return dapc.GetInfoString(ctx, 3)
}

func (dapc *dapClient) GetFirmwareVersion(ctx context.Context) (string, error) {
	return dapc.GetInfoString(ctx, 4)
}

func (dapc *dapClient) GetTargetVendor(ctx context.Context) (string, error) {
	return dapc.GetInfoString(ctx, 5)
}

func (dapc *dapClient) GetTargetName(ctx context.Context) (string, error) {
	return dapc.GetInfoString(ctx, 6)
}

func (dapc *dapClient) SetHostStatus(ctx context.Context, st StatusType, value bool) error {
	args := newCmd(cmdSetHostStatus)
	binary.Write(args, binary.LittleEndian, uint8(st))
	return errors.Trace(dapc.execCheckStatus(ctx, args))
}

func (dapc *dapClient) Connect(ctx context.Context, mode ConnectMode) error {
	glog.V(3).Infof("Connect(%d)", mode)
	args := newCmd(cmdConnect)
	binary.Write(args, binary.LittleEndian, uint8(mode))
	resp, err := dapc.exec(ctx, args)
	if err != nil {
		return errors.Trace(err)
	}
	if resp.Bytes()[0] == 0 {
		return errors.Errorf("connect error")
	}
	return nil
}

func (dapc *dapClient) Disconnect(ctx context.Context) error {
	return errors.Trace(dapc.execCheckStatus(ctx, newCmd(cmdDisconnect)))
}

func (dapc *dapClient) TransferConfigure(ctx context.Context, idleCycles uint8, waitRetry uint16, matchRetry uint16) error {
	glog.V(3).Infof("TransferConfigure(%d, %d, %d)", idleCycles, waitRetry, matchRetry)
	args := newCmd(cmdTransferConfigure)
	binary.Write(args, binary.LittleEndian, idleCycles)
	binary.Write(args, binary.LittleEndian, waitRetry)
	binary.Write(args, binary.LittleEndian, matchRetry)
	return errors.Trace(dapc.execCheckStatus(ctx, args))
}

func (dapc *dapClient) doTransfer(ctx context.Context, dapIndex uint8, reqs []TransferRequest) (TransferStatus, []uint32, error) {
	args := newCmd(cmdTransfer)
	binary.Write(args, binary.LittleEndian, dapIndex)
	binary.Write(args, binary.LittleEndian, uint8(len(reqs)))
	for i, req := range reqs {
		if req.Reg&3 != 0 {
			return 0, nil, errors.Errorf("treq %d invalid reg 0x%x", i, req.Reg)
		}
		treq := (req.Reg & 0xc)
		haveData := true
		if req.AP {
			treq |= 1 << 0
		}
		switch req.Op {
		case OpRead:
			treq |= 1 << 1
			haveData = false
		case OpReadMatch:
			treq |= 1<<1 | 1<<4
		case OpWrite:
			// Nothing
		case OpWriteMatch:
			treq |= 1 << 5
		}
		binary.Write(args, binary.LittleEndian, treq)
		if haveData {
			binary.Write(args, binary.LittleEndian, req.Data)
		}
	}
	resp, err := dapc.exec(ctx, args)
	if err != nil {
		return 0, nil, errors.Trace(err)
	}
	var tc uint8
	var st TransferStatus
	var data []uint32
	if binary.Read(resp, binary.LittleEndian, &tc) != nil ||
		binary.Read(resp, binary.LittleEndian, &st) != nil {
		return st, nil, errors.Errorf("response is too short")
	}
	if !st.Ok() {
		return st, nil, errors.Errorf("transfer failed (tc %d/%d st 0x%02x)", tc, len(reqs), st)
	}
	if int(tc) != len(reqs) {
		return st, nil, errors.Errorf("not all transfers completed", st)
	}
	for _, req := range reqs {
		if req.Op != OpRead {
			continue
		}
		var d uint32
		if binary.Read(resp, binary.LittleEndian, &d) != nil {
			return st, nil, errors.Errorf("response is too short")
		}
		data = append(data, d)
	}
	return st, data, nil
}

func (dapc *dapClient) Transfer(ctx context.Context, dapIndex uint8, reqs []TransferRequest) (TransferStatus, []uint32, error) {
	for i := 0; i < 5; i++ {
		st, res, err := dapc.doTransfer(ctx, dapIndex, reqs)
		if err != nil && st == TransferStatusWait {
			continue
		}
		return st, res, err
	}
	return TransferStatusWait, nil, errors.Errorf("transfer timeout")
}

func (dapc *dapClient) GetTransferBlockMaxSize() int {
	headerLen := 1 /* op */ + 1 /* dap index */ + 2 /* transfer count */ + 1 /* request */
	return (dapc.maxPacketSize - headerLen) / 4
}

func (dapc *dapClient) TransferBlockRead(ctx context.Context, dapIndex uint8, ap bool, reg uint8, length int) ([]uint32, error) {
	glog.V(3).Infof("TransferBlockRead(%d, %t, 0x%x, %d)", dapIndex, ap, reg, length)
	if length > dapc.GetTransferBlockMaxSize() {
		return nil, errors.Errorf("request too big (max %d, got %d)", dapc.GetTransferBlockMaxSize(), length)
	}
	args := newCmd(cmdTransferBlock)
	binary.Write(args, binary.LittleEndian, dapIndex)
	binary.Write(args, binary.LittleEndian, uint16(length))
	if reg&3 != 0 {
		return nil, errors.Errorf("invalid reg 0x%x", reg)
	}
	treq := uint8(reg&0xc) | 2 /* read */
	if ap {
		treq |= 1 << 0
	}
	binary.Write(args, binary.LittleEndian, treq)
	resp, err := dapc.exec(ctx, args)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var tc uint16
	var st TransferStatus
	if binary.Read(resp, binary.LittleEndian, &tc) != nil ||
		binary.Read(resp, binary.LittleEndian, &st) != nil {
		return nil, errors.Errorf("response is too short")
	}
	if !st.Ok() {
		return nil, errors.Errorf("transfer failed (tc %d/%d st 0x%02x)", tc, length, st)
	}
	if int(tc) != length {
		return nil, errors.Errorf("not all transfers completed", st)
	}
	var res []uint32
	for i := 0; i < length; i++ {
		var w uint32
		if binary.Read(resp, binary.LittleEndian, &w) != nil {
			return nil, errors.Errorf("response is too short")
		}
		res = append(res, w)
	}
	return res, nil
}

func (dapc *dapClient) TransferBlockWrite(ctx context.Context, dapIndex uint8, ap bool, reg uint8, data []uint32) error {
	glog.V(3).Infof("TransferBlockWrite(%d, %t, 0x%x, %d)", dapIndex, ap, reg, len(data))
	args := newCmd(cmdTransferBlock)
	binary.Write(args, binary.LittleEndian, dapIndex)
	binary.Write(args, binary.LittleEndian, uint16(len(data)))
	if reg&3 != 0 {
		return errors.Errorf("invalid reg 0x%x", reg)
	}
	treq := uint8(reg & 0xc)
	if ap {
		treq |= 1 << 0
	}
	binary.Write(args, binary.LittleEndian, treq)
	for _, value := range data {
		binary.Write(args, binary.LittleEndian, value)
	}
	resp, err := dapc.exec(ctx, args)
	if err != nil {
		return errors.Trace(err)
	}
	var tc uint16
	var st TransferStatus
	if binary.Read(resp, binary.LittleEndian, &tc) != nil ||
		binary.Read(resp, binary.LittleEndian, &st) != nil {
		return errors.Errorf("response is too short")
	}
	if !st.Ok() {
		return errors.Errorf("transfer failed (tc %d/%d st 0x%02x)", tc, len(data), st)
	}
	if int(tc) != len(data) {
		return errors.Errorf("not all transfers completed", st)
	}
	return nil
}

func (dapc *dapClient) Delay(ctx context.Context, delay time.Duration) error {
	delayMicros := delay.Nanoseconds() / 1000
	if delayMicros > 65535 {
		return errors.Errorf("delay too large (%d)", delayMicros)
	}
	glog.V(3).Infof("Delay(%d)", delayMicros)
	args := newCmd(cmdDelay)
	binary.Write(args, binary.LittleEndian, uint16(delayMicros))
	return errors.Trace(dapc.execCheckStatus(ctx, args))
}

func (dapc *dapClient) ResetTarget(ctx context.Context) error {
	return errors.Trace(dapc.execCheckStatus(ctx, newCmd(cmdResetTarget)))
}

func (dapc *dapClient) SWJClock(ctx context.Context, clockHz uint32) error {
	glog.V(3).Infof("SWJClock(%d)", clockHz)
	args := newCmd(cmdSWJClock)
	binary.Write(args, binary.LittleEndian, clockHz)
	return errors.Trace(dapc.execCheckStatus(ctx, args))
}

func (dapc *dapClient) SWJSequence(ctx context.Context, numBits int, data []uint8) error {
	glog.V(3).Infof("SWJSequence(%d, %v)", numBits, data)
	args := newCmd(cmdSWJSequence)
	if numBits < 1 || numBits > 256 {
		return errors.Errorf("length must be between and 256 (got %d)", len(data))
	}
	binary.Write(args, binary.LittleEndian, uint8(numBits))
	args.Write(data)
	return errors.Trace(dapc.execCheckStatus(ctx, args))
}

func (dapc *dapClient) SWDConfigure(ctx context.Context, config uint8) error {
	glog.V(3).Infof("SWDConfigure(0x%02x)", config)
	args := newCmd(cmdSWDConfigure)
	binary.Write(args, binary.LittleEndian, config)
	return errors.Trace(dapc.execCheckStatus(ctx, args))
}

func (dapc *dapClient) Close(ctx context.Context) error {
	if dapc.d != nil {
		dapc.d.Close()
	}
	return nil
}
