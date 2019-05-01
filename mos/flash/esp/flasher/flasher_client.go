package flasher

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"time"
	"unsafe"

	"cesanta.com/mos/flash/common"
	"cesanta.com/mos/flash/esp"
	"cesanta.com/mos/flash/esp/rom_client"
	"cesanta.com/mos/flash/esp32"
	"cesanta.com/mos/flash/esp8266"
	"github.com/cesanta/errors"
	"github.com/cesanta/go-serial/serial"
	"github.com/golang/glog"
)

const (
	FLASH_BLOCK_SIZE  = 65536
	FLASH_SECTOR_SIZE = 4096
	// These consts should be in sync with stub_flasher.c
	BUF_SIZE      = 4096
	UART_BUF_SIZE = 4 * BUF_SIZE
)

const (
	chipEraseTimeout  = 25 * time.Second
	blockEraseTimeout = 5 * time.Second
	// This is made small to workaround slow Mac driver
	flashReadSize = 256
)

/* Decls from stub_flasher.h */
type flasherCmd uint8

const (
	cmdFlashWrite      flasherCmd = 0x01
	cmdFlashRead                  = 0x02
	cmdFlashDigest                = 0x03
	cmdFlashReadChipID            = 0x04
	cmdFlashEraseChip             = 0x05
	cmdFlashBootFW                = 0x06
	cmdEcho                       = 0x08
	cmdReadReg                    = 0x09
	cmdWriteReg                   = 0x0a
)

type FlasherClient struct {
	ct        esp.ChipType
	s         serial.Serial
	srw       *common.SLIPReaderWriter
	rom       *rom_client.ROMClient
	connected bool
}

func NewFlasherClient(ct esp.ChipType, rc *rom_client.ROMClient, romBaudRate uint, baudRate uint) (*FlasherClient, error) {
	if baudRate < 0 || baudRate > 4000000 {
		return nil, errors.Errorf("invalid flashing baud rate (%d)", baudRate)
	}
	fc := &FlasherClient{ct: ct, s: rc.DataPort(), srw: common.NewSLIPReaderWriter(rc.DataPort()), rom: rc}
	if err := fc.connect(romBaudRate, baudRate); err != nil {
		return nil, errors.Trace(err)
	}
	return fc, nil
}

func (fc *FlasherClient) connect(romBaudRate, baudRate uint) error {
	var stubJSON []byte
	switch fc.ct {
	case esp.ChipESP8266:
		stubJSON = esp8266.MustAsset("data/stub_flasher.json")
	case esp.ChipESP32:
		stubJSON = esp32.MustAsset("data/stub_flasher.json")
	default:
		return errors.Errorf("unknown chip type %d", fc.ct)
	}

	common.Reportf("Running flasher @ %d...", baudRate)
	err := fc.rom.RunStub(stubJSON, []uint32{uint32(romBaudRate), uint32(baudRate)})
	if err != nil {
		return errors.Annotatef(err, "failed to run flasher stub")
	}
	if baudRate > 0 {
		glog.V(1).Infof("Switching to %d...", baudRate)
		if err := fc.s.SetBaudRate(baudRate); err != nil {
			return errors.Annotatef(err, "failed to set serial speed to %d", baudRate)
		}
	}
	fc.s.SetReadTimeout(50 * time.Millisecond)
	buf := make([]byte, 100)
	if n, err := fc.srw.Read(buf); err == nil && n == 4 {
		var oldUARTDiv uint32
		if binary.Read(bytes.NewBuffer(buf), binary.LittleEndian, &oldUARTDiv) == nil {
			oldUARTdivF := float64(oldUARTDiv)
			if fc.ct == esp.ChipESP32 {
				oldUARTdivF /= 16.0
			}
			masterCLK := uint32(romBaudRate) * uint32(oldUARTdivF)
			glog.V(1).Infof("Previous UART divider: %.3f (0x%x), master clk: %d", oldUARTdivF, oldUARTDiv, masterCLK)
		}
	}
	if err = fc.Sync(); err != nil {
		return errors.Annotatef(err, "failed to talk to flasher")
	}
	common.Reportf("  Flasher is running")
	fc.connected = true
	return nil
}

func (fc *FlasherClient) sendCommand(cmd flasherCmd, args []uint32) error {
	glog.V(2).Infof("%s %+v", cmd, args)
	buf := bytes.NewBuffer([]byte{byte(cmd)})
	_, err := fc.srw.Write(buf.Bytes())
	if err != nil {
		return errors.Annotatef(err, "failed to send command %d", cmd)
	}
	if len(args) > 0 {
		buf.Reset()
		for _, a := range args {
			binary.Write(buf, binary.LittleEndian, uint32(a))
		}
		_, err := fc.srw.Write(buf.Bytes())
		if err != nil {
			return errors.Annotatef(err, "failed to send args")
		}
	}
	return nil
}

func (fc *FlasherClient) recvResponse() ([][]byte, error) {
	var result [][]byte
	for {
		buf := make([]byte, 10000)
		n, err := fc.srw.Read(buf)
		if err != nil {
			return result, errors.Annotatef(err, "error reading response packet")
		}
		glog.V(3).Infof("<= %q", hex.EncodeToString(buf[:n]))
		// All responses end with one-byte status code.
		if n == 1 {
			if buf[0] != 0 {
				return result, errors.Errorf("error code: 0x%02x", buf[0])
			}
			return result, nil
		} else {
			r := make([]byte, n)
			copy(r, buf[:n])
			result = append(result, r)
		}
	}
}

func (fc *FlasherClient) simpleCmd(cmd flasherCmd, args []uint32, timeout time.Duration) ([][]byte, error) {
	if !fc.connected {
		return nil, errors.New("not connected")
	}
	err := fc.s.SetReadTimeout(timeout)
	if err != nil {
		return nil, errors.Annotatef(err, "failed to set read timeout")
	}
	if err = fc.sendCommand(cmd, args); err != nil {
		return nil, errors.Annotatef(err, "error sending command %d", cmd)
	}
	return fc.recvResponse()
}

func (fc *FlasherClient) GetFlashChipID() (uint32, error) {
	if !fc.connected {
		return 0, errors.New("not connected")
	}
	result, err := fc.simpleCmd(cmdFlashReadChipID, nil, 100*time.Millisecond)
	if err != nil {
		return 0, errors.Trace(err)
	}
	if len(result) != 1 {
		return 0, errors.Errorf("expected 1 result, got %d", len(result))
	}
	if len(result[0]) != 4 {
		return 0, errors.Errorf("expected 4 bytes, got %d", len(result[0]))
	}
	var chipID uint32
	binary.Read(bytes.NewBuffer(result[0]), binary.BigEndian, &chipID)
	if chipID == 0 {
		return 0, errors.New("failed to read chip id (0 is not a valid id)")
	}
	return (chipID >> 8), nil
}

func (fc *FlasherClient) EraseChip() error {
	if !fc.connected {
		return errors.New("not connected")
	}
	_, err := fc.simpleCmd(cmdFlashEraseChip, nil, chipEraseTimeout)
	return err
}

type writeProgress struct {
	numWritten uint32
	bufLevel   uint32
	digest     [md5.Size]byte
}

type writeResult struct {
	waitTime   uint32
	decompTime uint32
	writeTime  uint32
	eraseTime  uint32
	totalTime  uint32
	digest     [md5.Size]byte
}

func (fc *FlasherClient) Sync() error {
	fc.s.SetReadTimeout(50 * time.Millisecond)
sync:
	for i := 0; i < 5; i++ {
		cookie := rand.Uint32()
		fc.srw.Write(nil) // Send an empty packet to abort write, if any.
		if err := fc.sendCommand(cmdEcho, []uint32{cookie}); err != nil {
			return errors.Trace(err)
		}
		buf := make([]byte, 1024)
		n := 0
		for {
			if n >= len(buf) {
				n = 0
			}
			nr, err := fc.s.Read(buf[n:])
			if err != nil {
				continue sync
			}
			glog.V(3).Infof("<= %s", hex.EncodeToString(buf[n:n+nr]))
			n += nr
			echoData := bytes.NewBuffer(nil)
			binary.Write(echoData, binary.LittleEndian, cookie)
			if bytes.Contains(buf, echoData.Bytes()) {
				glog.Infof("re-synced")
				return nil
			}
		}
	}
	return errors.Errorf("flasher did not respond")
}

func (fc *FlasherClient) Write(addr uint32, data []byte, erase bool, compress bool) (int, error) {
	var numSent, numWritten, numBytesOnTheWire int
	if !fc.connected {
		return numWritten, errors.New("not connected")
	}
	eraseFlag := uint32(0)
	if erase {
		eraseFlag = 1
	}
	fc.s.SetReadTimeout(blockEraseTimeout)
	err := fc.sendCommand(cmdFlashWrite, []uint32{addr, uint32(len(data)), eraseFlag})
	if err != nil {
		return numWritten, errors.Trace(err)
	}
	digest := md5.New()
	for numWritten < len(data) {
		var progress writeProgress
		buf := make([]byte, unsafe.Sizeof(progress))
		n, err := fc.srw.Read(buf)
		if err != nil {
			return numWritten, errors.Annotatef(err, "flash write failed @ %d/%d", numWritten, numSent)
		}
		if n < len(buf) {
			return numWritten, errors.Errorf("invalid write progress packet %q", hex.EncodeToString(buf[:n]))
		}
		bb := bytes.NewBuffer(buf)
		binary.Read(bb, binary.LittleEndian, &progress.numWritten)
		binary.Read(bb, binary.LittleEndian, &progress.bufLevel)
		bb.Read(progress.digest[:])
		newNumWritten := int(progress.numWritten)
		if newNumWritten < numWritten || newNumWritten > len(data) {
			return numWritten, errors.Errorf("bad write progress packet %q", hex.EncodeToString(buf[:n]))
		}
		if newNumWritten != numWritten {
			digest.Write(data[numWritten:newNumWritten])
		}
		expectedDigest := digest.Sum(nil)
		expectedDigestHex := strings.ToLower(hex.EncodeToString(expectedDigest[:]))
		digestHex := strings.ToLower(hex.EncodeToString(progress.digest[:]))
		if numWritten > 0 && digestHex != expectedDigestHex {
			return numWritten, errors.Errorf("digest mismatch @ %d: expected %s, got %s", numWritten, expectedDigestHex, digestHex)
		}
		numWritten = newNumWritten
		inFlight := numSent - numWritten
		canSend := int(UART_BUF_SIZE - BUF_SIZE - inFlight)
		glog.V(3).Infof("<= %d %d; %d/%d/%d; %s", numWritten, progress.bufLevel, numSent, inFlight, canSend, digestHex)
		for numSent < len(data) && canSend > 0 {
			numToSend := len(data) - numSent
			if numToSend > canSend {
				numToSend = canSend
			}
			if numToSend > BUF_SIZE {
				numToSend = BUF_SIZE
			}
			toSend := data[numSent : numSent+numToSend]
			var compressed bytes.Buffer
			if compress {
				// Try compressing, see if it gets smaller
				w, _ := zlib.NewWriterLevel(&compressed, zlib.BestCompression)
				w.Write(toSend)
				w.Close()
				glog.V(4).Infof("%d -> %d", len(toSend), compressed.Len())
			}
			if compress && compressed.Len() < len(toSend) {
				toSend2 := make([]byte, 0, compressed.Len()+1)
				toSend2 = append(toSend2, 0x01)
				toSend = append(toSend2, compressed.Bytes()...)
			} else {
				toSend2 := make([]byte, 0, len(toSend)+1)
				toSend2 = append(toSend2, 0x00)
				toSend = append(toSend2, toSend...)
			}
			ns, err := fc.srw.Write(toSend)
			if err != nil {
				return numWritten, errors.Annotatef(err, "flash write failed @ %d/%d", numWritten, numSent)
			}
			numSent += numToSend
			canSend -= numToSend
			numBytesOnTheWire += ns
			glog.V(3).Infof("=> %d; %d/%d/%d", ns, numWritten, numSent, len(data))
		}
	}
	var result writeResult
	tail, err := fc.recvResponse()
	if err != nil {
		return numWritten, errors.Annotatef(err, "failed to read digest and stats")
	}
	if len(tail) != 1 || len(tail[0]) != int(unsafe.Sizeof(result)) {
		return numWritten, errors.Errorf("unexpected digest packet %+v", tail)
	}
	sdb := bytes.NewBuffer(tail[0])
	binary.Read(sdb, binary.LittleEndian, &result.waitTime)
	binary.Read(sdb, binary.LittleEndian, &result.decompTime)
	binary.Read(sdb, binary.LittleEndian, &result.writeTime)
	binary.Read(sdb, binary.LittleEndian, &result.eraseTime)
	binary.Read(sdb, binary.LittleEndian, &result.totalTime)
	sdb.Read(result.digest[:])
	expectedDigest := md5.Sum(data)
	expectedDigestHex := strings.ToLower(hex.EncodeToString(expectedDigest[:]))
	digestHex := strings.ToLower(hex.EncodeToString(result.digest[:]))
	if digestHex != expectedDigestHex {
		return numWritten, errors.Errorf("final digest mismatch: expected %s, got %s", expectedDigestHex, digestHex)
	}
	miscTime := result.totalTime - result.waitTime - result.decompTime - result.eraseTime - result.writeTime
	glog.Infof("Write stats: waitTime:%.2f decompTime:%.2f writeTime:%.2f eraseTime:%.2f miscTime:%.2f totalTime:%d compFactor:%.2f",
		float64(result.waitTime)/float64(result.totalTime),
		float64(result.decompTime)/float64(result.totalTime),
		float64(result.writeTime)/float64(result.totalTime),
		float64(result.eraseTime)/float64(result.totalTime),
		float64(miscTime)/float64(result.totalTime),
		result.totalTime,
		float64(numBytesOnTheWire)/float64(numWritten),
	)
	return numWritten, nil
}

func (fc *FlasherClient) Read(addr uint32, data []byte) error {
	if !fc.connected {
		return errors.New("not connected")
	}
	err := fc.sendCommand(cmdFlashRead, []uint32{
		addr, uint32(len(data)), flashReadSize, flashReadSize})
	if err != nil {
		return errors.Trace(err)
	}
	numRead := 0
	for numRead < len(data) {
		buf := data[numRead:]
		if len(buf) > flashReadSize {
			buf = buf[:flashReadSize]
		}
		n, err := fc.srw.Read(buf)
		if err != nil {
			return errors.Annotatef(err, "flash read failed @ 0x%x", numRead)
		}
		if n != len(buf) {
			return errors.Errorf("unexpected result packet length %d", n)
		}
		numRead += len(buf)
		nrb := bytes.NewBuffer(nil)
		binary.Write(nrb, binary.LittleEndian, uint32(numRead))
		fc.srw.Write(nrb.Bytes())
		glog.V(3).Infof("<= %d; %d/%d", len(buf), numRead, len(data))
	}
	tail, err := fc.recvResponse()
	if err != nil {
		return errors.Annotatef(err, "failed to read digest")
	}
	if len(tail) != 1 || len(tail[0]) != md5.Size {
		return errors.Errorf("unexpected digest packet %+v", tail)
	}
	expectedDigest := md5.Sum(data)
	expectedDigestHex := strings.ToLower(hex.EncodeToString(expectedDigest[:]))
	digestHex := strings.ToLower(hex.EncodeToString(tail[0]))
	if digestHex != expectedDigestHex {
		return errors.Errorf("digest mismatch: expected %s, got %s", expectedDigestHex, digestHex)
	}
	return nil
}

func (fc *FlasherClient) ReadReg(addr uint32) (uint32, error) {
	if !fc.connected {
		return 0, errors.New("not connected")
	}
	result, err := fc.simpleCmd(cmdReadReg, []uint32{addr}, 1*time.Second)
	if err != nil {
		return 0, errors.Trace(err)
	}
	if len(result) != 1 || len(result[0]) != 4 {
		return 0, errors.Errorf("invalid response to ReadReg: %v", result)
	}
	var res uint32
	binary.Read(bytes.NewBuffer(result[0]), binary.LittleEndian, &res)
	return res, nil
}

func (fc *FlasherClient) WriteReg(addr uint32, value uint32) error {
	if !fc.connected {
		return errors.New("not connected")
	}
	_, err := fc.simpleCmd(cmdWriteReg, []uint32{addr, value}, 1*time.Second)
	return errors.Trace(err)
}

func (fc *FlasherClient) Digest(addr, length, blockSize uint32) ([][]byte, error) {
	if !fc.connected {
		return nil, errors.New("not connected")
	}
	timeout := 2 * time.Second
	if blockSize == 0 {
		timeout += time.Duration(2*length/1048576) * time.Second
	}
	if timeout > 24*time.Second {
		timeout = 24 * time.Second
	}
	result, err := fc.simpleCmd(cmdFlashDigest, []uint32{addr, length, blockSize}, timeout)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return result, nil
}

func (fc *FlasherClient) BootFirmware() error {
	// So, this is a bit tricky. Rebooting ESP8266 "properly" from software
	// seems to be impossible due to GPIO strapping: at this point we have
	// STRAPPING_GPIO0 = 0 and as far as we are aware it's not possible to
	// perform a reset that will cause strapping bits to be re-initialized.
	// Jumping to ResetVector or perforing RTC reset (bit 31 in RTC_CTL)
	// simply gets us back into boot loader.
	// flasher_client performs a "soft" reboot, which simply jumps to the
	// routine that loads fw. This will work even if RTS and DTR are not connected,
	// but the side effect is that firmware will not be able to reboot properly.
	// So, what we do is we do both: tell the flasher to boot firmware *and*
	// tickle RTS as well. Thus, setups that have control lines connected will
	// get a "proper" hardware reset, while setups that don't will still work.
	_, err := fc.simpleCmd(cmdFlashBootFW, nil, 1*time.Second) // Jumps to the flash loader routine.
	fc.rom.BootFirmware()                                      // Performs hw reset using RTS, if possible.
	return err
}

func (fc *FlasherClient) Disconnect() {
}

// TODO(rojer): Use stringer when it actually works.
func (cmd flasherCmd) String() string {
	switch cmd {
	case cmdFlashWrite:
		return fmt.Sprintf("FlashWrite(%d)", cmd)
	case cmdFlashRead:
		return fmt.Sprintf("FlashRead(%d)", cmd)
	case cmdFlashDigest:
		return fmt.Sprintf("FlashDigest(%d)", cmd)
	case cmdFlashReadChipID:
		return fmt.Sprintf("FlashReadChipID(%d)", cmd)
	case cmdFlashEraseChip:
		return fmt.Sprintf("FlashEraseChip(%d)", cmd)
	case cmdFlashBootFW:
		return fmt.Sprintf("FlashBootFW(%d)", cmd)
	case cmdEcho:
		return fmt.Sprintf("Echo(%d)", cmd)
	default:
		return fmt.Sprintf("?(%d)", cmd)
	}
}
