package cortex

import (
	"context"
	"fmt"

	"github.com/cesanta/errors"
	"github.com/golang/glog"

	"cesanta.com/mos/flash/common"
)

type CortexDebug interface {
	common.Target

	Init(ctx context.Context) error
}

const (
	regCPUID    uint32 = 0xE000ED00
	regAIRCR           = 0xE000ED0C
	regAIRCRKey        = 0x05FA0000

	regDHCSR    = 0xE000EDF0
	regDHCSRKey = 0xA05F0000
	regDCRSR    = 0xE000EDF4
	regDCRDR    = 0xE000EDF8
	regDEMCR    = 0xE000EDFC
	regPID0     = 0xE000EFE0
)

type CortexRegFile struct {
	R    [16]uint32
	XPSR uint32
	MSP  uint32
	PSP  uint32
}

const SP = 13 // SP is an alias for R13
const LR = 14 // LR is an alias for R14
const PC = 15 // LR is an alias for R15

func TargetName(cpuid, pid0 uint32) string {
	glog.V(1).Infof("CPUID: 0x%08x, PID0: 0x%08x", cpuid, pid0)
	vendorno := cpuid >> 24
	vendor := ""
	switch vendorno {
	case 0x41:
		vendor = "ARM"
	}
	patch := cpuid & 0xf
	partno := (cpuid >> 4) & 0xfff
	rev := (cpuid >> 20) & 0xf
	part := ""
	switch partno {
	case 0xc20:
		part = "Cortex-M0"
	case 0xc60:
		part = "Cortex-M0+"
	case 0xc21:
		part = "Cortex-M1"
	case 0xc23:
		part = "Cortex-M3"
	case 0xc24:
		part = "Cortex-M4"
	case 0xc27:
		part = "Cortex-M7"
	}
	fpu := ""
	if pid0 == 0xc {
		fpu = "F"
	}
	return fmt.Sprintf("%s %s%s r%dp%d", vendor, part, fpu, rev, patch)
}

func GetTargetName(ctx context.Context, tmrw common.TargetMemReaderWriter) (string, error) {
	cpuid, err := tmrw.ReadTargetReg(ctx, regCPUID)
	if err != nil {
		return "", errors.Annotatef(err, "failed to get CPUID")
	}
	pid0, err := tmrw.ReadTargetReg(ctx, regPID0)
	if err != nil {
		return "", errors.Annotatef(err, "failed to get PID0")
	}
	return TargetName(cpuid, pid0), nil
}

func (r CortexRegFile) String() string {
	return fmt.Sprintf(
		"[R0=0x%x R1=0x%x R2=0x%x R3=0x%x R4=0x%x R5=0x%x R6=0x%x R7=0x%x "+
			"R8=0x%x R9=0x%x R10=0x%x R11=0x%x R12=0x%x SP=0x%x LR=0x%x PC=0x%x xPSR=0x%x MSP=0x%x PSP=0x%x]",
		r.R[0], r.R[1], r.R[2], r.R[3], r.R[4], r.R[5], r.R[6], r.R[7], r.R[8], r.R[9], r.R[10], r.R[11], r.R[12],
		r.R[SP], r.R[LR], r.R[PC], r.XPSR, r.MSP, r.PSP)
}
