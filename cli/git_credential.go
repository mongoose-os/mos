package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/build"
	"github.com/mongoose-os/mos/cli/dev"
	glog "k8s.io/klog/v2"
)

func gitCredentials(ctx context.Context, devConn dev.DevConn) error {
	creds, err := getCredentialsFromCLI()
	if err != nil {
		return err
	}
	if len(creds) == 0 {
		return errors.Errorf("git-credential requires --gh-token")
	}
	// The protocol is documented in git-credential(1):
	// https://git-scm.com/docs/git-credential
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), "=", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[0] != "host" {
			continue
		}
		host := parts[1]
		c := build.GetCredentialsForHost(creds, host)
		if c != nil {
			glog.Infof("Found credentials for %q", host)
			fmt.Fprintln(os.Stdout, fmt.Sprintf("username=%s", c.User))
			fmt.Fprintln(os.Stdout, fmt.Sprintf("password=%s", c.Pass))
		} else {
			glog.Infof("No credentials found for %q", host)
		}
		return nil
	}
	return nil
}
