package cortex

import (
	"context"

	"github.com/cesanta/errors"
	"github.com/golang/glog"

	"cesanta.com/mos/flash/common"
)

// Doc: ARM v7-M Architecture Reference Manual

func NewCM4Debug(tmrw common.TargetMemReaderWriter) CortexDebug {
	return &cm4Debug{tmrw: tmrw}
}

type cm4Debug struct {
	tmrw common.TargetMemReaderWriter
}

func (cm4d *cm4Debug) Init(ctx context.Context) error {
	cpuid, err := cm4d.tmrw.ReadTargetReg(ctx, regCPUID)
	if err != nil {
		return errors.Annotatef(err, "failed to get CPUID")
	}
	if cpuid&0xff00fff0 != 0x4100c240 {
		return errors.Errorf("target is not a Cortex-M4 (CPUID 0x%08x)", cpuid)
	}
	return nil
}

func (cm4d *cm4Debug) reset(ctx context.Context, dhcsr, demcr uint32) error {
	if err := cm4d.tmrw.WriteTargetReg(ctx, regDHCSR, dhcsr); err != nil {
		return errors.Annotatef(err, "failed to set DHCSR")
	}
	if err := cm4d.tmrw.WriteTargetReg(ctx, regDEMCR, demcr); err != nil {
		return errors.Annotatef(err, "failed to set DEMCR")
	}
	return cm4d.tmrw.WriteTargetReg(ctx, regAIRCR, regAIRCRKey|0x4 /* SYSRESETREQ */)
}

func (cm4d *cm4Debug) ResetHalt(ctx context.Context) error {
	// Per RM C1.4.1: set DHCSR.C_DEBUGEN, DEMCR.VC_CORERESET (and other traps) and reset.
	if err := cm4d.reset(ctx, regDHCSRKey|1, 0x3f1); err != nil {
		return errors.Annotatef(err, "failed to reset the core")
	}
	return cm4d.WaitHalt(ctx)
}

func (cm4d *cm4Debug) ResetRun(ctx context.Context) error {
	// Reset with debug disabled.
	return cm4d.reset(ctx, regDHCSRKey|0, 0)
}

func (cm4d *cm4Debug) WaitHalt(ctx context.Context) error {
	for {
		dhcsr, err := cm4d.tmrw.ReadTargetReg(ctx, regDHCSR)
		if err != nil {
			return errors.Annotatef(err, "failed to get DHCSR")
		}
		glog.V(3).Infof("WaitHalt DHCSR 0x%08x", dhcsr)
		if dhcsr&2 != 0 { // C_HALT
			break
		}
	}
	return nil
}

func (cm4d *cm4Debug) waitRegReady(ctx context.Context) error {
	for {
		dhcsr, err := cm4d.tmrw.ReadTargetReg(ctx, regDHCSR)
		if err != nil {
			return errors.Annotatef(err, "failed to get DHCSR")
		}
		if dhcsr&(1<<16) != 0 {
			break
		}
	}
	return nil
}

func (cm4d *cm4Debug) SetReg(ctx context.Context, reg int, value uint32) error {
	glog.V(4).Infof("SetReg(%d, 0x%x)", reg, value)
	if err := cm4d.tmrw.WriteTargetReg(ctx, regDCRDR, value); err != nil {
		return errors.Annotatef(err, "failed to set DCRDR")
	}
	if err := cm4d.tmrw.WriteTargetReg(ctx, regDCRSR, (1<<16)|uint32(reg)); err != nil {
		return errors.Annotatef(err, "failed to set DCRSR")
	}
	return errors.Trace(cm4d.waitRegReady(ctx))
}

func (cm4d *cm4Debug) SetRegs(ctx context.Context, regFile interface{}) error {
	// ARMv7 RM C1.6.3
	regs, ok := regFile.(*CortexRegFile)
	if !ok {
		return errors.Errorf("invalid reg file format")
	}
	glog.V(3).Infof("SetRegs(%s)", regs)
	for i := 0; i < 16; i++ {
		if err := cm4d.SetReg(ctx, i, regs.R[i]); err != nil {
			return errors.Annotatef(err, "failed to set R%d", i)
		}
	}
	if err := cm4d.SetReg(ctx, 0x10, regs.XPSR); err != nil {
		return errors.Annotatef(err, "failed to set xPSR")
	}
	if err := cm4d.SetReg(ctx, 0x11, regs.MSP); err != nil {
		return errors.Annotatef(err, "failed to set MSP")
	}
	if err := cm4d.SetReg(ctx, 0x12, regs.PSP); err != nil {
		return errors.Annotatef(err, "failed to set PSP")
	}
	return nil
}

func (cm4d *cm4Debug) getReg(ctx context.Context, reg uint32, valuePtr *uint32) error {
	if err := cm4d.tmrw.WriteTargetReg(ctx, regDCRSR, reg); err != nil {
		return errors.Annotatef(err, "failed to set DCRSR")
	}
	if err := errors.Trace(cm4d.waitRegReady(ctx)); err != nil {
		return errors.Annotatef(err, "failed to wait for reg read")
	}
	value, err := cm4d.tmrw.ReadTargetReg(ctx, regDCRDR)
	if err != nil {
		return errors.Annotatef(err, "failed to read DCRDR")
	}
	glog.V(4).Infof("GetReg(%d) == 0x%x", reg, value)
	*valuePtr = value
	return nil
}

func (cm4d *cm4Debug) GetReg(ctx context.Context, reg int) (uint32, error) {
	var value uint32
	if err := cm4d.getReg(ctx, uint32(reg), &value); err != nil {
		return 0, errors.Trace(err)
	}
	return value, nil
}

func (cm4d *cm4Debug) GetRegs(ctx context.Context, regFile interface{}) error {
	glog.V(3).Infof("GetRegs()")
	regs, ok := regFile.(*CortexRegFile)
	if !ok {
		return errors.Errorf("invalid reg file format")
	}
	for i := 0; i < 16; i++ {
		if err := cm4d.getReg(ctx, uint32(i), &regs.R[i]); err != nil {
			return errors.Annotatef(err, "failed to get R%d", i)
		}
	}
	if err := cm4d.getReg(ctx, 0x10, &regs.XPSR); err != nil {
		return errors.Annotatef(err, "failed to get xPSR")
	}
	if err := cm4d.getReg(ctx, 0x11, &regs.MSP); err != nil {
		return errors.Annotatef(err, "failed to get MSP")
	}
	if err := cm4d.getReg(ctx, 0x12, &regs.PSP); err != nil {
		return errors.Annotatef(err, "failed to get PSP")
	}
	glog.V(3).Infof("Regs: %s", regs)
	return nil
}

func (cm4d *cm4Debug) Run(ctx context.Context, waitHalt bool) error {
	glog.V(3).Infof("Run(%t)", waitHalt)
	if err := cm4d.tmrw.WriteTargetReg(ctx, regDHCSR, regDHCSRKey|1); err != nil {
		return errors.Annotatef(err, "failed to set DHCSR")
	}
	return errors.Trace(cm4d.WaitHalt(ctx))
}
