// Copyright (c) 2014-2017 Cesanta Software Limited
// All rights reserved

package mosgit

import (
	"flag"
	"runtime"

	"cesanta.com/common/go/ourgit"
	"github.com/golang/glog"
)

var (
	useShellGitFlag = flag.Bool("use-shell-git", false, "use external git binary instead of internal implementation")
	useGoGitFlag    = flag.Bool("use-go-git", false, "use internal Git library (go-git)")

	haveShellGit    = false
	checkedShellGit = false
)

// NewOurGit returns an instance of OurGit: if --use-shell-git is given it'll
// be a shell-based implementation; otherwise a go-git-based one.
func NewOurGit() ourgit.OurGit {
	if *useShellGitFlag {
		return ourgit.NewOurGitShell()
	} else if *useGoGitFlag {
		return ourgit.NewOurGit()
	}
	// User did not express a preference.
	// In that case, prefer shell Git to go-git (if available).
	if !checkedShellGit {
		haveShellGit = ourgit.HaveShellGit()
		if haveShellGit {
			glog.Infof("Found Git executable, using shell Git")
		} else {
			glog.Infof("No Git executable found, using go-git")
		}
		checkedShellGit = true
	}
	if haveShellGit {
		return ourgit.NewOurGitShell()
	} else {
		return ourgit.NewOurGit()
	}
}

func IsClean(gitinst ourgit.OurGit, localDir, version string) (bool, error) {
	var excl []string
	// This kludge excludes fetched binary libs.
	// It should be removed when we no longer fetch libs into the repo itself.
	excl = append(excl, "lib/*/*.a")
	if runtime.GOOS == "windows" {
		// And this one is a workaround for https://github.com/src-d/go-git/issues/378.
		excl = append(excl, "*.pl", "*.py", "*.sh", "rom.elf", "user.h", "*.version")
	}
	return gitinst.IsClean(localDir, version, excl)
}
