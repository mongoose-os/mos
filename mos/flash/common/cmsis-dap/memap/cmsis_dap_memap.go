package memap

import (
	"context"
	"fmt"

	"github.com/cesanta/errors"
	"github.com/golang/glog"

	"cesanta.com/mos/flash/common"
	"cesanta.com/mos/flash/common/cmsis-dap/dp"
)

type MemAPReg uint8

const (
	CSW  MemAPReg = 0x00
	TAR           = 0x04
	DRW           = 0x0c
	BD0           = 0x10
	BD1           = 0x14
	BD2           = 0x18
	BD3           = 0x1c
	BASE          = 0xf8
)

type MemAPClient interface {
	common.TargetMemReaderWriter

	Init(ctx context.Context) error
	ReadReg(ctx context.Context, reg MemAPReg) (uint32, error)
	WriteReg(ctx context.Context, reg MemAPReg, value uint32) error
}

type memAPClient struct {
	dpc   dp.DPClient
	apSel uint8
}

func NewMemAPClient(dpc dp.DPClient, apSel uint8) MemAPClient {
	return &memAPClient{dpc: dpc, apSel: apSel}
}

const (
	CSW_DeviceEn = 0x40
)

func (mapc *memAPClient) ReadReg(ctx context.Context, reg MemAPReg) (uint32, error) {
	value, err := mapc.dpc.ReadAPReg(ctx, mapc.apSel, uint8(reg))
	glog.V(4).Infof("%s == 0x%08x", reg, value)
	return value, err
}

func (mapc *memAPClient) WriteReg(ctx context.Context, reg MemAPReg, value uint32) error {
	glog.V(4).Infof("%s = 0x%08x", reg, value)
	return mapc.dpc.WriteAPReg(ctx, mapc.apSel, uint8(reg), value)
}

func (mapc *memAPClient) Init(ctx context.Context) error {
	csw, err := mapc.ReadReg(ctx, CSW)
	if err != nil {
		return errors.Trace(err)
	}
	if csw&CSW_DeviceEn == 0 {
		return errors.Errorf("MEM-AP is disabled")
	}
	return mapc.WriteReg(ctx, CSW, 0x23000052) // Basic mode, word access, increment by 1.
}

func (mapc *memAPClient) ReadTargetReg(ctx context.Context, addr uint32) (uint32, error) {
	if err := mapc.WriteReg(ctx, TAR, addr); err != nil {
		return 0, errors.Trace(err)
	}
	value, err := mapc.ReadReg(ctx, DRW)
	glog.V(4).Infof("ReadTargetReg(0x%08x) == 0x%08x", addr, value)
	return value, errors.Trace(err)
}

func (mapc *memAPClient) ReadTargetMem(ctx context.Context, addr uint32, length int) ([]uint32, error) {
	if err := mapc.WriteReg(ctx, TAR, addr); err != nil {
		return nil, errors.Trace(err)
	}
	values, err := mapc.dpc.ReadAPRegMulti(ctx, mapc.apSel, uint8(DRW), length)
	glog.V(4).Infof("ReadTargetMem(0x%08x, %d)", addr, length)
	return values, errors.Trace(err)
}

func (mapc *memAPClient) WriteTargetReg(ctx context.Context, addr uint32, value uint32) error {
	if err := mapc.WriteReg(ctx, TAR, addr); err != nil {
		return errors.Trace(err)
	}
	glog.V(4).Infof("WriteTargetReg(0x%08x, 0x%08x)", addr, value)
	return mapc.WriteReg(ctx, DRW, value)
}

func (mapc *memAPClient) WriteTargetMem(ctx context.Context, addr uint32, data []uint32) error {
	if err := mapc.WriteReg(ctx, TAR, addr); err != nil {
		return errors.Trace(err)
	}
	glog.V(4).Infof("WriteTargetMem(0x%08x, %d)", addr, len(data))
	return mapc.dpc.WriteAPRegMulti(ctx, mapc.apSel, uint8(DRW), data)
}

func (r MemAPReg) String() string {
	switch r {
	case CSW:
		return "CSW"
	case TAR:
		return "TAR"
	case DRW:
		return "DRW"
	case BD0:
		return "BD0"
	case BD1:
		return "BD1"
	case BD2:
		return "BD2"
	case BD3:
		return "BD3"
	case BASE:
		return "BASE"
	}
	return fmt.Sprintf("0x%x", uint8(r))
}
