package atca

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"context"

	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/dev"
	"github.com/cesanta/errors"
)

const (
	KeyFilePrefix = "ATCA:"
)

func Connect(ctx context.Context, dc dev.DevConn) ([]byte, *Config, error) {
	var r GetConfigResult

	if err := dc.Call(ctx, "ATCA.GetConfig", nil, &r); err != nil {
		return nil, nil, errors.Annotatef(err, "GetConfig")
	}

	if r.Config == nil {
		return nil, nil, errors.New("no config data in response")
	}

	confData, err := base64.StdEncoding.DecodeString(*r.Config)
	if err != nil {
		return nil, nil, errors.Annotatef(err, "failed to decode config data")
	}
	if len(confData) != ConfigSize {
		return nil, nil, errors.Errorf("expected %d bytes, got %d", ConfigSize, len(confData))
	}

	cfg, err := ParseBinaryConfig(confData)
	if err != nil {
		return nil, nil, errors.Annotatef(err, "ParseBinaryConfig")
	}

	var model string
	if cfg.Revision >= 0x6000 {
		model = "ATECC608A"
	} else {
		model = "ATECC508A"
	}
	ourutil.Reportf("\n%s rev 0x%x S/N 0x%s, config is %s, data is %s",
		model, cfg.Revision, hex.EncodeToString(cfg.SerialNum), strings.ToLower(string(cfg.LockConfig)),
		strings.ToLower(string(cfg.LockValue)))

	if cfg.LockConfig != LockModeLocked || cfg.LockValue != LockModeLocked {
		ourutil.Reportf("WARNING: Either config or data zone are not locked, chip is not fully configured")
	}
	ourutil.Reportf("")

	return confData, cfg, nil
}

func WriteHex(data []byte, numPerLine int) []byte {
	s := ""
	for i := 0; i < len(data); {
		for j := 0; j < numPerLine && i < len(data); j++ {
			comma := ""
			if i < len(data)-1 {
				comma = ", "
			}
			s += fmt.Sprintf("0x%02x%s", data[i], comma)
			i++
		}
		s += "\n"
	}
	return []byte(s)
}

func ReadHex(data []byte) []byte {
	var result []byte
	hexByteRegex := regexp.MustCompile(`[0-9a-fA-F]{2}`)
	for _, match := range hexByteRegex.FindAllString(string(data), -1) {
		b, _ := hex.DecodeString(match)
		result = append(result, b[0])
	}
	return result
}

func JSONStr(v interface{}) string {
	bb, _ := json.MarshalIndent(v, "", "  ")
	return string(bb)
}
