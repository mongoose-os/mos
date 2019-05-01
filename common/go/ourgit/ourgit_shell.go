// Copyright (c) 2014-2017 Cesanta Software Limited
// All rights reserved

package ourgit

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"path"
	"strings"

	"github.com/cesanta/errors"
	"github.com/golang/glog"
)

type ourGitShell struct{}

func (m *ourGitShell) GetCurrentHash(localDir string) (string, error) {
	resp, err := shellGit(localDir, "rev-parse", "HEAD")
	if err != nil {
		return "", errors.Annotatef(err, "failed to get current hash")
	}
	if len(resp) == 0 {
		return "", errors.Errorf("failed to get current hash")
	}
	return resp, nil
}

func (m *ourGitShell) DoesBranchExist(localDir string, branch string) (bool, error) {
	resp, err := shellGit(localDir, "branch", "--list", branch)
	if err != nil {
		return false, errors.Annotatef(err, "failed to check if branch %q exists", branch)
	}
	return len(resp) > 2 && resp[2:] == branch, nil
}

func (m *ourGitShell) DoesTagExist(localDir string, tag string) (bool, error) {
	resp, err := shellGit(localDir, "tag", "--list", tag)
	if err != nil {
		return false, errors.Annotatef(err, "failed to check if tag %q exists", tag)
	}
	return resp == tag, nil
}

func (m *ourGitShell) GetToplevelDir(localDir string) (string, error) {
	resp, err := shellGit(localDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", errors.Annotatef(err, "failed to get git toplevel dir")
	}
	return resp, nil
}

func (m *ourGitShell) Checkout(localDir string, id string, refType RefType) error {
	_, err := shellGit(localDir, "checkout", id)
	if err != nil {
		return errors.Annotatef(err, "failed to git checkout %s", id)
	}
	return nil
}

func (m *ourGitShell) Pull(localDir string) error {
	_, err := shellGit(localDir, "pull", "--all")
	if err != nil {
		return errors.Annotatef(err, "failed to git pull")
	}
	return nil
}

func (m *ourGitShell) Fetch(localDir string, opts FetchOptions) error {
	args := []string{"--tags"}

	if opts.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", opts.Depth))
	}

	_, err := shellGit(localDir, "fetch", args...)
	if err != nil {
		return errors.Annotatef(err, "failed to git fetch")
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
	resp, err := shellGit(localDir, "ls-files", flags...)
	if err != nil {
		return false, errors.Annotatef(err, "failed to git ls-files")
	}

	if resp != "" {
		// Working dir is dirty
		glog.Errorf("%s: dirty: %s", localDir, resp)
		return false, nil
	}

	// Unfortunately, git ls-files is unable to show staged and uncommitted files.
	// So, specifically for these files, we'll have to run git diff --cached:

	resp, err = shellGit(localDir, "diff", "--cached", "--name-only")
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
		glog.Errorf("%s: dirty 2: %s", localDir, resp)
		return false, nil
	}

	// Working directory is clean. Are we on a particular tag?
	resp, err = shellGit(localDir, "describe", "--tags", "--exact-match", "HEAD")
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

	resp, err = shellGit(localDir, "cherry")
	if err != nil {
		// Apparently the repo is not on a branch, retry with the version
		resp, err = shellGit(localDir, "cherry", version)
		if err != nil {
			// We can get an error at this point if given version does not exist
			// in the repository; in this case assume the repo is clean
			return true, nil
		}
	}

	if resp != "" {
		// Some commits need to be pushed to upstream
		glog.Errorf("%s: dirty 3", localDir)
		return false, nil
	}

	// Working dir is clean
	return true, nil
}

func (m *ourGitShell) ResetHard(localDir string) error {
	_, err := shellGit(localDir, "checkout", ".")
	if err != nil {
		return errors.Annotatef(err, "failed to git checkout .")
	}
	return nil
}

func (m *ourGitShell) Clone(srcURL, targetDir string, opts CloneOptions) error {
	args := []string{"clone"}

	if opts.ReferenceDir != "" {
		args = append(args, "--reference", opts.ReferenceDir)
	}

	if opts.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", opts.Depth))
	}

	if opts.Ref != "" {
		args = append(args, "-b", opts.Ref)
	}

	var berr bytes.Buffer
	args = append(args, srcURL, targetDir)
	cmd := exec.Command("git", args...)
	cmd.Stderr = &berr

	err := cmd.Run()
	if err != nil {
		return errors.Annotatef(err, "cloning %s: %s", srcURL, berr.String())
	}

	return nil
}

func (m *ourGitShell) GetOriginUrl(localDir string) (string, error) {
	resp, err := shellGit(localDir, "remote", "get-url", "origin")
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

func shellGit(localDir string, subcmd string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{subcmd}, args...)...)

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
	// What we need is os/exec.LookupPath("git"), but Go 1.10 doesn't have it yet.
	_, err := shellGit("", "version")
	return err == nil
}
