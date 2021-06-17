// Copyright (c) 2014-2017 Cesanta Software Limited
// All rights reserved

package ourgit

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/juju/errors"
	glog "k8s.io/klog/v2"
)

const (
	GitAskPassEnv   = "GIT_ASKPASS"
	gitCredsUserEnv = "_MOS_GIT_CREDS_USER"
	gitCredsPassEnv = "_MOS_GIT_CREDS_PASS"
)

// NewOurGit returns a shell-based implementation of OurGit
// (external git binary is required for that to work)
func NewOurGitShell(creds *Credentials) OurGit {
	return &ourGitShell{creds: creds}
}

type ourGitShell struct {
	creds *Credentials
}

func (m *ourGitShell) GetCurrentHash(localDir string) (string, error) {
	resp, err := m.shellGit(localDir, "rev-parse", "HEAD")
	if err != nil {
		return "", errors.Annotatef(err, "failed to get current hash")
	}
	if len(resp) == 0 {
		return "", errors.Errorf("failed to get current hash")
	}
	return resp, nil
}

func (m *ourGitShell) DoesBranchExist(localDir string, branch string) (bool, error) {
	resp, err := m.shellGit(localDir, "branch", "--list", branch)
	if err != nil {
		return false, errors.Annotatef(err, "failed to check if branch %q exists", branch)
	}
	return len(resp) > 2 && resp[2:] == branch, nil
}

func (m *ourGitShell) DoesTagExist(localDir string, tag string) (bool, error) {
	resp, err := m.shellGit(localDir, "tag", "--list", tag)
	if err != nil {
		return false, errors.Annotatef(err, "failed to check if tag %q exists", tag)
	}
	return resp == tag, nil
}

func (m *ourGitShell) GetToplevelDir(localDir string) (string, error) {
	resp, err := m.shellGit(localDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", errors.Annotatef(err, "failed to get git toplevel dir")
	}
	return resp, nil
}

func (m *ourGitShell) Checkout(localDir string, id string, refType RefType) error {
	_, err := m.shellGit(localDir, "checkout", id)
	if err != nil {
		return errors.Annotatef(err, "failed to git checkout %s", id)
	}
	return nil
}

func (m *ourGitShell) Pull(localDir string, branch string) error {
	_, err := m.shellGit(localDir, "pull", "origin", branch)
	if err != nil {
		return errors.Annotatef(err, "failed to git pull")
	}
	return nil
}

func (m *ourGitShell) Fetch(localDir string, what string, opts FetchOptions) error {
	args := []string{"--tags"}

	if opts.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", opts.Depth))
	}

	args = append(args, "origin", fmt.Sprintf("%s:%s", what, what))

	_, err := m.shellGit(localDir, "fetch", args...)
	if err != nil {
		return errors.Annotatef(err, "failed to git fetch %s %s", localDir, what)
	}
	return nil
}

// IsClean returns true if there are no modified, deleted or untracked files,
// and no non-pushed commits since the given version.
func (m *ourGitShell) IsClean(localDir, version string, excludeGlobs []string) (bool, error) {
	// First, check if there are modified, deleted or untracked files
	flags := []string{"--exclude-standard", "--modified", "--others", "--deleted"}
	for _, g := range excludeGlobs {
		flags = append(flags, fmt.Sprintf("--exclude=%s", g))
	}
	resp, err := m.shellGit(localDir, "ls-files", flags...)
	if err != nil {
		return false, errors.Annotatef(err, "failed to git ls-files")
	}

	if resp != "" {
		// Working dir is dirty
		glog.Errorf("%s: dirty (uncommited changes present)", localDir)
		return false, nil
	}

	// Unfortunately, git ls-files is unable to show staged and uncommitted files.
	// So, specifically for these files, we'll have to run git diff --cached:

	resp, err = m.shellGit(localDir, "diff", "--cached", "--name-only")
	if err != nil {
		return false, errors.Annotatef(err, "failed to git diff --cached")
	}

	s := bufio.NewScanner(bytes.NewBuffer([]byte(resp)))
scan:
	for s.Scan() {
		fn := s.Text()
		for _, g := range excludeGlobs {
			m1, _ := path.Match(g, fn)
			m2, _ := path.Match(g, path.Base(fn))
			if m1 || m2 {
				continue scan
			}
		}
		glog.Errorf("%s: dirty (untracked files or uncommitted changes)", localDir)
		return false, nil
	}

	// Working directory is clean. Are we on a particular tag?
	resp, err = m.shellGit(localDir, "describe", "--tags", "--exact-match", "HEAD")
	if err == nil {
		// We are. That's cool, we can manage the repo then.
		return true, nil
	}

	// Working directory is clean, now we need to check if there are some
	// non-pushed commits. Unfortunately there is no way (that I know of) which
	// would work with both branches and tags. So, we do this:
	//
	// Invoke "git cherry". If the repo is on a branch, this command will print
	// list of commits to be pushed to upstream. If, however, the repo is not on
	// a branch (e.g. it's often on a tag), then this command will fail, and in
	// that case we invoke it again, but with the version specified:
	// "git cherry <version>". In either case, non-empty output means the
	// precense of some commits which would not be fetched by the remote builder,
	// so the repo is dirty.

	resp, err = m.shellGit(localDir, "cherry")
	if err != nil {
		// Apparently the repo is not on a branch, retry with the version
		resp, err = m.shellGit(localDir, "cherry", version)
		if err != nil {
			// We can get an error at this point if given version does not exist
			// in the repository; in this case assume the repo is clean
			return true, nil
		}
	}

	if resp != "" {
		// Some commits need to be pushed to upstream
		glog.Errorf("%s: dirty (unpushed commits present)", localDir)
		return false, nil
	}

	// Working dir is clean
	return true, nil
}

func (m *ourGitShell) ResetHard(localDir string) error {
	_, err := m.shellGit(localDir, "checkout", ".")
	if err != nil {
		return errors.Annotatef(err, "failed to git checkout .")
	}
	return nil
}

func (m *ourGitShell) Clone(srcURL, targetDir string, opts CloneOptions) error {
	var args []string

	if opts.ReferenceDir != "" {
		args = append(args, "--reference", opts.ReferenceDir)
	}

	if opts.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", opts.Depth))
	}

	if opts.Ref != "" {
		args = append(args, "-b", opts.Ref)
	}

	args = append(args, srcURL, targetDir)

	_, err := m.shellGit("", "clone", args...)

	return errors.Trace(err)
}

func (m *ourGitShell) GetOriginURL(localDir string) (string, error) {
	resp, err := m.shellGit(localDir, "remote", "get-url", "origin")
	if err != nil {
		return "", errors.Annotatef(err, "failed to get origin URL")
	}
	if len(resp) == 0 {
		return "", errors.Errorf("failed to get origin URL")
	}
	return resp, nil
}

func (m *ourGitShell) HashesEqual(hash1, hash2 string) bool {
	minLen := len(hash1)
	if len(hash2) < minLen {
		minLen = len(hash2)
	}

	// Check if at least one of the hashes is too short
	if minLen < minHashLen {
		return false
	}

	return hash1[:minLen] == hash2[:minLen]
}

func (m *ourGitShell) shellGit(localDir string, subcmd string, args ...string) (string, error) {
	var cmdArgs []string

	// If the user provided credentials, insert ourserves as auth helper.
	if m.creds != nil {
		if myPath, err := os.Executable(); err == nil {
			myPath = strings.Replace(myPath, `\`, `\\`, -1)
			myPath = strings.Replace(myPath, ` `, `\ `, -1)
			cmdArgs = append(cmdArgs,
				"-c", fmt.Sprintf("credential.helper=%s --log_file=/dev/null git-credentials --credentials=:%s:%s",
					myPath, m.creds.User, m.creds.Pass))
		}
	}

	cmdArgs = append(cmdArgs, subcmd)
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command("git", cmdArgs...)

	var b bytes.Buffer
	var berr bytes.Buffer
	cmd.Dir = localDir
	cmd.Stdout = &b
	cmd.Stderr = &berr
	err := cmd.Run()
	if err != nil {
		return "", errors.Annotate(err, berr.String())
	}
	resp := b.String()
	return strings.TrimRight(resp, "\r\n"), nil
}

// HaveShellGit checks if "git" command is available.
func HaveShellGit() bool {
	_, err := exec.LookPath("git")
	return err == nil
}
