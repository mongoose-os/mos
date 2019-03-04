package dp

import (
	"context"
	"fmt"

	"github.com/cesanta/errors"
	"github.com/golang/glog"

	"cesanta.com/mos/flash/common/cmsis-dap/dap"
)

type DPReg uint8

const (
	DPIDR      DPReg = 0x00
	DPCTRLSTAT       = 0x04
	DPSELECT         = 0x08
)

type DPClient interface {
	Init(ctx context.Context) error
	GetIDR(ctx context.Context) (DPIDRValue, error)
	DbgReset(ctx context.Context) error
	SetDbgPower(ctx context.Context, dbg, sys bool) error
	ReadDPReg(ctx context.Context, reg DPReg) (uint32, error)
	WriteDPReg(ctx context.Context, reg DPReg, value uint32) error
	ReadAPReg(ctx context.Context, apSel, apReg uint8) (uint32, error)
	ReadAPRegMulti(ctx context.Context, apSel, apReg uint8, length int) ([]uint32, error)
	WriteAPReg(ctx context.Context, apSel, apReg uint8, value uint32) error
	WriteAPRegMulti(ctx context.Context, apSel, apReg uint8, values []uint32) error
}

func NewDPClient(dapc dap.DAPClient) DPClient {
	return &dpClient{dapc: dapc}
}

type dpClient struct {
	dapc dap.DAPClient

	selectValue uint32
}

func (dpc *dpClient) ReadReg(ctx context.Context, reg uint8, ap bool) (uint32, error) {
	_, data, err := dpc.dapc.Transfer(ctx, 0, []dap.TransferRequest{
		dap.TransferRequest{Op: dap.OpRead, AP: ap, Reg: reg},
	})
	if err != nil {
		return 0, errors.Annotatef(err, "failed to read DP reg %d", reg)
	}
	return data[0], nil
}

func (dpc *dpClient) ReadRegMulti(ctx context.Context, reg uint8, ap bool, length int) ([]uint32, error) {
	maxChunkSize := dpc.dapc.GetTransferBlockMaxSize()
	var res []uint32
	for length > 0 {
		chunkSize := length
		if chunkSize > maxChunkSize {
			chunkSize = maxChunkSize
		}
		chunk, err := dpc.dapc.TransferBlockRead(ctx, 0, ap, reg, chunkSize)
		if err != nil {
			return nil, errors.Trace(err)
		}
		res = append(res, chunk...)
		length -= chunkSize
	}
	return res, nil
}

func (dpc *dpClient) ReadDPReg(ctx context.Context, reg DPReg) (uint32, error) {
	value, err := dpc.ReadReg(ctx, uint8(reg), false /* ap */)
	glog.V(4).Infof("%s == 0x%08x", reg, value)
	return value, err
}

func (dpc *dpClient) WriteReg(ctx context.Context, reg uint8, ap bool, value uint32) error {
	_, _, err := dpc.dapc.Transfer(ctx, 0, []dap.TransferRequest{
		dap.TransferRequest{Op: dap.OpWrite, AP: ap, Reg: reg, Data: value},
	})
	return err
}

func (dpc *dpClient) WriteRegMulti(ctx context.Context, reg uint8, ap bool, values []uint32) error {
	offset := 0
	maxChunkSize := dpc.dapc.GetTransferBlockMaxSize()
	for offset < len(values) {
		chunk := values[offset:]
		if len(chunk) > maxChunkSize {
			chunk = chunk[:maxChunkSize]
		}
		if err := dpc.dapc.TransferBlockWrite(ctx, 0, ap, reg, chunk); err != nil {
			return errors.Trace(err)
		}
		offset += len(chunk)
	}
	return nil
}

func (dpc *dpClient) WriteDPReg(ctx context.Context, reg DPReg, value uint32) error {
	glog.V(4).Infof("%s = 0x%08x", reg, value)
	return errors.Trace(dpc.WriteReg(ctx, uint8(reg), false /* ap */, value))
}

func (dpc *dpClient) Init(ctx context.Context) error {
	if _, err := dpc.GetIDR(ctx); err != nil {
		return errors.Annotatef(err, "failed to read DP ID")
	}
	if err := dpc.WriteDPReg(ctx, DPSELECT, 0); err != nil {
		return errors.Trace(err)
	}
	dpc.selectValue = 0
	if err := dpc.SetDbgPower(ctx, true, true); err != nil {
		return errors.Trace(err)
	}
	// Clear all the errors (if any).
	if err := dpc.WriteDPReg(ctx, DPCTRLSTAT, 0x50000f00); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (dpc *dpClient) GetIDR(ctx context.Context) (DPIDRValue, error) {
	v, err := dpc.ReadDPReg(ctx, DPIDR)
	if err != nil {
		return 0, errors.Annotatef(err, "failed to read DPIDR")
	}
	return DPIDRValue(v), nil
}

func (dpc *dpClient) SetDbgPower(ctx context.Context, dbg, sys bool) error {
	var reqMask, ackMask uint32
	if dbg {
		reqMask |= 0x10000000
		ackMask |= 0x20000000
	}
	if sys {
		reqMask |= 0x40000000
		ackMask |= 0x80000000
	}
	for {
		statValue, err := dpc.ReadDPReg(ctx, DPCTRLSTAT)
		if err != nil {
			return errors.Annotatef(err, "failed to read DPCTRLSTAT")
		}
		if statValue&0xf0000000 == (reqMask | ackMask) {
			break
		}
		ctrlValue := (statValue & 0x07ffffff) | reqMask
		if err := dpc.WriteDPReg(ctx, DPCTRLSTAT, ctrlValue); err != nil {
			return errors.Annotatef(err, "failed to write DPCTRLSTAT")
		}
	}
	return nil
}

func (dpc *dpClient) DbgReset(ctx context.Context) error {
	statValue, err := dpc.ReadDPReg(ctx, DPCTRLSTAT)
	if err != nil {
		return errors.Annotatef(err, "failed to read DPCTRLSTAT")
	}
	// Set reset request
	ctrlValue := (statValue & 0xf3ffffff) | 0x04000000
	if err := dpc.WriteDPReg(ctx, DPCTRLSTAT, ctrlValue); err != nil {
		return errors.Annotatef(err, "failed to write DPCTRLSTAT")
	}
	// Wait for ack
	for {
		statValue, err = dpc.ReadDPReg(ctx, DPCTRLSTAT)
		if err != nil {
			return errors.Annotatef(err, "failed to read DPCTRLSTAT")
		}
		if statValue&0x08000000 != 0 {
			break
		}
	}
	// Remove request
	ctrlValue = (statValue & 0xf3ffffff)
	if err := dpc.WriteDPReg(ctx, DPCTRLSTAT, ctrlValue); err != nil {
		return errors.Annotatef(err, "failed to write DPCTRLSTAT")
	}
	// Wait for ack to clear
	for {
		statValue, err = dpc.ReadDPReg(ctx, DPCTRLSTAT)
		if err != nil {
			return errors.Annotatef(err, "failed to read DPCTRLSTAT")
		}
		if statValue&0x08000000 == 0 {
			break
		}
	}
	return nil
}

func (dpc *dpClient) selectAP(ctx context.Context, apSel, apBank uint8) error {
	sv := (dpc.selectValue & 0x00ffff0f) | (uint32(apSel) << 24) | ((uint32(apBank) & 0xf) << 4)
	if sv == dpc.selectValue {
		return nil
	}
	if err := dpc.WriteDPReg(ctx, DPSELECT, sv); err != nil {
		return errors.Annotatef(err, "failed to select AP %d bank %d", apSel, apBank)
	}
	dpc.selectValue = sv
	return nil
}

func (dpc *dpClient) ReadAPReg(ctx context.Context, apSel, apReg uint8) (uint32, error) {
	apBank := apReg / 16
	if err := dpc.selectAP(ctx, apSel, apBank); err != nil {
		return 0, errors.Trace(err)
	}
	apReg = apReg % 16
	return dpc.ReadReg(ctx, apReg, true /* ap */)
}

func (dpc *dpClient) ReadAPRegMulti(ctx context.Context, apSel, apReg uint8, length int) ([]uint32, error) {
	apBank := apReg / 16
	if err := dpc.selectAP(ctx, apSel, apBank); err != nil {
		return nil, errors.Trace(err)
	}
	apReg = apReg % 16
	return dpc.ReadRegMulti(ctx, apReg, true /* ap */, length)
}

func (dpc *dpClient) WriteAPReg(ctx context.Context, apSel, apReg uint8, value uint32) error {
	apBank := apReg / 16
	if err := dpc.selectAP(ctx, apSel, apBank); err != nil {
		return errors.Trace(err)
	}
	apReg = apReg % 16
	return dpc.WriteReg(ctx, apReg, true /* ap */, value)
}

func (dpc *dpClient) WriteAPRegMulti(ctx context.Context, apSel, apReg uint8, values []uint32) error {
	apBank := apReg / 16
	if err := dpc.selectAP(ctx, apSel, apBank); err != nil {
		return errors.Trace(err)
	}
	apReg = apReg % 16
	return dpc.WriteRegMulti(ctx, apReg, true /* ap */, values)
}

type DPIDRValue uint32

type DPDesigner uint16

func (v DPIDRValue) Designer() DPDesigner {
	return DPDesigner(v & 0xfff)
}

func (v DPIDRValue) Version() uint8 {
	return uint8((v >> 12) & 0xf)
}

func (v DPIDRValue) Minimal() bool {
	return (v>>16)&1 != 0
}

func (v DPIDRValue) PartNumber() bool {
	return (v>>24)&0xf != 0
}

func (v DPIDRValue) Revision() uint8 {
	return uint8((v >> 28) & 0xf)
}

func (v DPDesigner) String() string {
	if v == 0x477 {
		return "ARM"
	}
	return fmt.Sprintf("0x%03x", uint16(v))
}

func (r DPReg) String() string {
	switch r {
	case DPIDR:
		return "DPIDR"
	case DPCTRLSTAT:
		return "DPCTRLSTAT"
	case DPSELECT:
		return "DPSELECT"
	}
	return fmt.Sprintf("0x%x", uint8(r))
}
