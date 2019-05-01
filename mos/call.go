package main

import (
	"context"
	"encoding/json"
	"fmt"

	"cesanta.com/mos/dev"
	"cesanta.com/mos/flags"

	"github.com/cesanta/errors"
	flag "github.com/spf13/pflag"
)

func isJSONString(s string) bool {
	var js string
	return json.Unmarshal([]byte(s), &js) == nil
}

func isJSON(s string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}

func callDeviceService(
	ctx context.Context, devConn dev.DevConn, method string, args string,
) (string, error) {
	b, e := devConn.(*dev.MosDevConn).CallB(ctx, method, args)

	// TODO(dfrank): instead of that, we should probably add a separate function
	// for rebooting
	if method == "Sys.Reboot" {
		waitForReboot()
	}

	return string(b), e
}

func call(ctx context.Context, devConn dev.DevConn) error {
	args := flag.Args()[1:]
	if len(args) < 1 {
		return errors.Errorf("method required")
	}

	params := ""
	if len(args) > 1 {
		params = args[1]
	}

	if *flags.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *flags.Timeout)
		defer cancel()
	}

	result, err := callDeviceService(ctx, devConn, args[0], params)
	if err != nil {
		return err
	}

	fmt.Println(result)
	return nil
}
