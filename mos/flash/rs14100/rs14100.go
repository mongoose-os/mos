package rs14100

//go:generate go-bindata -pkg rs14100 -nocompress -modtime 1 -mode 420 data/

import (
	"bytes"
	"context"
	"encoding/binary"
	"time"

	"github.com/cesanta/errors"
	"github.com/golang/glog"

	"cesanta.com/common/go/fwbundle"
	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/flash/common"
	"cesanta.com/mos/flash/common/cmsis-dap/dap"
	"cesanta.com/mos/flash/common/cmsis-dap/dp"
	"cesanta.com/mos/flash/common/cmsis-dap/memap"
	"cesanta.com/mos/flash/common/cortex"
)

const (
	// RPS debug probe uses NXP VID. Go figure.
	vid   = 0x0d28
	pid   = 0x0204
	intf  = 0
	epIn  = 0x81
	epOut = 0x01
)

const (
	flasherAsset        = "data/RS14100_SF_4MB.FLM.bin"
	flasherBaseAddr     = 0x00000000
	flasherStack        = 0x0004000
	flasherWriteBufAddr = 0x0010000
	flasherWriteBufSize = 0x0010000
	// prefix length, applied to all function offsets.
	flasherFuncOffset = 56
	// Function offsets (as shown by objdump).
	// int Init(unsigned long adr, unsigned long clk, unsigned long fnc); // fnc: 1=erase, 2=prog, 3=verify
	flasherFuncInit = 0x00000357
	// int UnInit(unsigned long fnc);
	flasherFuncUnInit = 0x00000361
	// int EraseChip(void);
	flasherFuncEraseChip = 0x00000001
	// int EraseSector(unsigned long adr);
	flasherFuncEraseSector = 0x0000036b
	// int ProgramPage (unsigned long adr, unsigned long sz, unsigned char *buf);
	flasherFuncProgramPage = 0x00000379
	// unsigned long Verify(unsigned long adr, unsigned long sz, unsigned char *buf);
	flasherFuncVerify = 0x0000038d
)

const (
	flashPageSize   = 256
	flashSectorSize = 0x1000
	flashBase       = 0x8000000
)

type FlashDevice struct {
	Vers     uint16    // Version Number and Architecture
	DevName  [128]byte // Device Name and Description
	DevType  uint16    // Device Type: ONCHIP, EXT8BIT, EXT16BIT, ...
	DevAdr   uint32    // Default Device Start Address
	DevSize  uint32    // Total Size of Device
	PageSize uint32    // Programming Page Size
	Rsvd     uint32    // Reserved for future Extension
	ValEmpty uint8     // Content of Erased Memory

	ProgTimeoutMs  uint32 // Time Out of Program Page Function
	EraseTimeoutMs uint32 // Time Out of Erase Sector Function

	Sectors [512]FlashSector
}

type FlashSector struct {
	Size    uint32
	Address uint32
}

type FlashOpts struct {
	EraseChip bool
}

func runFlasherFunc(ctx2 context.Context, tgt common.Target, funcAddr uint32, args []uint32, timeout time.Duration) error {
	ctx := ctx2
	if timeout != 0 {
		ctx3, cancel := context.WithTimeout(ctx, timeout)
		ctx2 = ctx3
		defer cancel()
	}
	for i := 0; i < len(args); i++ {
		if err := tgt.SetReg(ctx, i, args[i]); err != nil {
			return errors.Annotatef(err, "failed to set arg %d", i)
		}
	}
	// There's a breakpoint instruction at the very beginning.
	if err := tgt.SetReg(ctx, cortex.LR, 1); err != nil {
		return errors.Annotatef(err, "failed to set LR")
	}
	if err := tgt.SetReg(ctx, cortex.PC, funcAddr+flasherFuncOffset); err != nil {
		return errors.Annotatef(err, "failed to set LR")
	}
	if err := tgt.Run(ctx, true /* waitHalt */); err != nil {
		return errors.Trace(err)
	}
	r0, err := tgt.GetReg(ctx, 0)
	if err != nil {
		return errors.Trace(err)
	}
	if r0 != 0 {
		return errors.Errorf("flasher failed (rv %d)", r0)
	}
	return nil
}

func toWords(data []byte, padWith byte) []uint32 {
	var w uint32
	var dataWords []uint32
	if len(data)%4 != 0 {
		data2 := make([]byte, (len(data)+4) & ^3)
		copy(data2, data)
		data = data2
	}
	fb := bytes.NewBuffer(data)
	for binary.Read(fb, binary.LittleEndian, &w) == nil {
		dataWords = append(dataWords, w)
	}
	return dataWords
}

func Flash(fw *fwbundle.FirmwareBundle, opts *FlashOpts) error {
	ctx := context.Background()
	dapc, err := dap.NewClient(ctx, vid, pid, "", intf, epIn, epOut)
	if err != nil {
		return errors.Annotatef(err, "failed open debug probe")
	}
	defer dapc.Close(context.Background())
	defer func() {
		dapc.Disconnect(context.Background())
	}()
	vendor, err := dapc.GetVendorID(ctx)
	product, _ := dapc.GetProductID(ctx)
	serial, _ := dapc.GetSerialNumber(ctx)
	version, _ := dapc.GetFirmwareVersion(ctx)
	targetVendor, _ := dapc.GetTargetVendor(ctx)
	targetName, _ := dapc.GetTargetName(ctx)
	ourutil.Reportf("CMSIS-DAP probe %s %s %s v%s S/N %s; target %s %s",
		vendor, product, serial, version, serial, targetVendor, targetName)
	if err := dapc.Connect(ctx, dap.ConnectModeSWD); err != nil {
		return errors.Annotatef(err, "failed to connect to debug probe in SWD mode")
	}
	// SWD init
	if err := dapc.SWJClock(ctx, 10000000); err != nil {
		return errors.Annotatef(err, "failed to set clock")
	}
	if err := dapc.SWDConfigure(ctx, 0); err != nil {
		return errors.Annotatef(err, "failed to configure SWD")
	}
	// Put into reset first (50+ of 1, 8+ of 0)
	if err := dapc.SWJSequence(ctx, 64, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}); err != nil {
		return errors.Annotatef(err, "SWD reset sequence failed")
	}
	if err := dapc.SWJSequence(ctx, 16, []byte{0, 0}); err != nil {
		return errors.Annotatef(err, "SWD reset sequence failed")
	}
	// Send JTAG-to-SWD switch sequence
	if err := dapc.SWJSequence(ctx, 64, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}); err != nil {
		return errors.Annotatef(err, "SWD reset sequence failed")
	}
	if err := dapc.SWJSequence(ctx, 16, []byte{0x9e, 0xe7}); err != nil {
		return errors.Annotatef(err, "SWD reset sequence failed")
	}
	// Reset again
	if err := dapc.SWJSequence(ctx, 64, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}); err != nil {
		return errors.Annotatef(err, "SWD reset sequence failed")
	}
	if err := dapc.SWJSequence(ctx, 16, []byte{0, 0}); err != nil {
		return errors.Annotatef(err, "SWD reset sequence failed")
	}
	if err := dapc.TransferConfigure(ctx, 0, 100, 100); err != nil {
		return errors.Annotatef(err, "failed to configure transfers")
	}
	dpc := dp.NewDPClient(dapc)
	if err := dpc.Init(ctx); err != nil {
		return errors.Annotatef(err, "failed to init DP, is the target connected and powered on?")
	}
	dpidr, err := dpc.GetIDR(ctx)
	if err != nil {
		return errors.Annotatef(err, "failed to read DP ID")
	}
	mapc := memap.NewMemAPClient(dpc, 0 /* apSel */)
	if err := mapc.Init(ctx); err != nil {
		return errors.Annotatef(err, "failed to init AP")
	}
	tgtName, err := cortex.GetTargetName(ctx, mapc)
	if err != nil {
		return errors.Annotatef(err, "failed to get target name")
	}
	ourutil.Reportf("Core: %s, DP v%d rev%d (%s), minimal? %t",
		tgtName, dpidr.Version(), dpidr.Revision(), dpidr.Designer(), dpidr.Minimal())
	cm4d := cortex.NewCM4Debug(mapc)
	if err := cm4d.Init(ctx); err != nil {
		return errors.Annotatef(err, "failed to init CM4 debug")
	}
	if err := dapc.SetHostStatus(ctx, dap.StatusConnected, true); err == nil {
		defer dapc.SetHostStatus(context.Background(), dap.StatusConnected, false)
	}
	ourutil.Reportf("Uploading flasher stub...")
	if err := cm4d.ResetHalt(ctx); err != nil {
		return errors.Annotatef(err, "failed to reset-halt the target")
	}

	flasherData := toWords(MustAsset(flasherAsset), 0)
	if err := mapc.WriteTargetMem(ctx, flasherBaseAddr, flasherData); err != nil {
		return errors.Annotatef(err, "failed to upload flasher stub")
	}
	verifyFlasherData, err := mapc.ReadTargetMem(ctx, flasherBaseAddr, len(flasherData))
	if err != nil {
		return errors.Annotatef(err, "failed to read back flasher stub")
	}
	for i, w := range flasherData {
		if verifyFlasherData[i] != w {
			return errors.Errorf("failed to read back flasher stub (%d 0x%08x 0x%08x)", i, verifyFlasherData[i], w)
		}
	}
	// Init registers
	regs := cortex.CortexRegFile{
		XPSR: 0x1000000,
		MSP:  flasherStack,
		PSP:  0x0,
	}
	if err := cm4d.SetRegs(ctx, &regs); err != nil {
		return errors.Trace(err)
	}
	var tErase, tSend, tWrite time.Duration
	start := time.Now()
	erasedChip := false
	if opts.EraseChip {
		eraseStart := time.Now()
		glog.V(1).Infof("Running Init(erase)...")
		if err := runFlasherFunc(ctx, cm4d, flasherFuncInit, []uint32{0x8012000, 12000000, 1}, 1*time.Second); err != nil {
			return errors.Annotatef(err, "failed to init flasher")
		}
		ourutil.Reportf("Erasing chip...")
		if err := runFlasherFunc(ctx, cm4d, flasherFuncEraseChip, nil, 60*time.Second); err != nil {
			return errors.Annotatef(err, "failed to erase chip")
		}
		tErase += time.Since(eraseStart)
		erasedChip = true
	}
	for _, p := range fw.PartsByAddr() {
		pData, err := p.GetData()
		if err != nil {
			return errors.Annotatef(err, "failed to get part %q data", p.Name)
		}
		for len(pData)%flashPageSize != 0 {
			pData = append(pData, 0xff)
		}
		pAddr := p.Addr
		if pAddr < flashBase {
			pAddr += flashBase
		}
		if !erasedChip {
			ea := pAddr
			es := (len(pData) + flashSectorSize - 1) & ^(flashSectorSize - 1)
			glog.V(1).Infof("Running Init(erase)...")
			if err := runFlasherFunc(ctx, cm4d, flasherFuncInit, []uint32{0x8012000, 12000000, 1}, 1*time.Second); err != nil {
				return errors.Annotatef(err, "failed to init flasher")
			}
			ourutil.Reportf("Erasing %d @ 0x%x...", es, ea)
			eraseStart := time.Now()
			for es > 0 {
				glog.V(1).Infof("Erasing %d @ 0x%x...", flashSectorSize, ea)
				if err := runFlasherFunc(ctx, cm4d, flasherFuncEraseSector, []uint32{ea}, 2*time.Second); err != nil {
					return errors.Annotatef(err, "failed to erase sector @ 0x%x", ea)
				}
				es -= flashSectorSize
				ea += flashSectorSize
			}
			tErase += time.Since(eraseStart)
		}
		wa := int(pAddr)
		ourutil.Reportf("Writing %d @ 0x%x...", len(pData), wa)
		glog.V(1).Infof("Running Init(write)...")
		if err := runFlasherFunc(ctx, cm4d, flasherFuncInit, []uint32{0x8012000, 12000000, 2}, 1*time.Second); err != nil {
			return errors.Annotatef(err, "failed to init flasher")
		}
		for wi := 0; wi < len(pData); {
			wl := len(pData) - wi
			if wl > flasherWriteBufSize {
				wl = flasherWriteBufSize
			}
			glog.V(1).Infof("Sending %d to 0x%x...", wl, flasherWriteBufAddr)
			sendStart := time.Now()
			data := toWords(pData[wi:wi+wl], 0xff)
			if err := mapc.WriteTargetMem(ctx, flasherWriteBufAddr, data); err != nil {
				return errors.Annotatef(err, "failed to upload data @ 0x%x", wi)
			}
			tSend += time.Since(sendStart)
			writeStart := time.Now()
			glog.V(1).Infof("Writing %d @ 0x%x...", wl, wa)
			if err := runFlasherFunc(ctx, cm4d, flasherFuncProgramPage, []uint32{uint32(wa), uint32(wl), uint32(flasherWriteBufAddr)}, 5*time.Second); err != nil {
				return errors.Annotatef(err, "failed to write @ 0x%x", wa)
			}
			tWrite += time.Since(writeStart)
			wa += wl
			wi += wl
		}
	}
	tAll := time.Since(start)
	glog.Infof("Took %.3f (%.3f erase, %.3f send, %.3f write)",
		tAll.Seconds(), tErase.Seconds(), tSend.Seconds(), tWrite.Seconds())
	ourutil.Reportf("Done! Running firmware...")
	if err := cm4d.ResetRun(ctx); err != nil {
		return errors.Annotatef(err, "failed to reset-run the target")
	}
	return nil
}

func init() {
	if len(MustAsset(flasherAsset))%4 != 0 {
		panic("flasher stub length must be 0 mod 4")
	}
}
