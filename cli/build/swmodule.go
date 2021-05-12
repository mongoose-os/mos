//
// Copyright (c) 2014-2019 Cesanta Software Limited
// All rights reserved
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
package build

import (
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/juju/errors"
	glog "k8s.io/klog/v2"

	"github.com/mongoose-os/mos/cli/mosgit"
	"github.com/mongoose-os/mos/cli/ourutil"
	"github.com/mongoose-os/mos/common/ourgit"
)

type SWModule struct {
	Type     string `yaml:"type,omitempty" json:"type,omitempty"`
	Name     string `yaml:"name,omitempty" json:"name,omitempty"`
	Location string `yaml:"location,omitempty" json:"location,omitempty"`
	// Origin is deprecated since 2017/08/18
	OriginOld string `yaml:"origin,omitempty" json:"origin,omitempty"`
	Version   string `yaml:"version,omitempty" json:"version,omitempty"`
	Variant   string `yaml:"variant,omitempty" json:"variant,omitempty"`

	// API used to download binary assets. If not specified, will take a guess based on location.
	AssetAPI SWModuleAssetAPIType `yaml:"asset_api,omitempty" json:"asset_api,omitempty"`

	versionOverride string

	localPath   string // Path where the lib resides locally. Valid after successful PrepareLocalDir.
	repoVersion string // Specific version (hash, commit) of the library at the localPath.
	isDirty     bool   // Local repo is "dirty" - i.e., has local changes.

	// Credential must be provided externally and never serialized in a manifest.
	credentials *Credentials
}

type SWModuleAssetAPIType string

const (
	AssetAPIGitHub SWModuleAssetAPIType = "github"
	AssetAPIGitLab                      = "gitlab"
)

type SWModuleType int

const (
	SWModuleTypeInvalid SWModuleType = iota
	SWModuleTypeLocal
	SWModuleTypeGit
)

var (
	gitSSHShortRegex = regexp.MustCompile(`^(?:(\w+)@)?(\S+?):(\S+)`)
	validNameRegex   = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-_]`)
	// If the lib is called "boards", we silently rename it to "zz_boards".
	// The goal here is to naturally sink this lib, which contains various overrides
	// for board configuration, down to the bottom of the list of libs so that
	// overrides actually have something to override.
	// It is for all intents and purposes equivalent to the following stanza under libs:
	//   - origin: https://github.com/mongoose-os-libs/boards
	//     name: zz_boards
	// ...but without the ugly and cryptic second line.
	// I'm not proud of this hack, but it's better than the alternatives.
	boardsLibName    = "boards"
	boardsLibNewName = "zz_boards"
)

func parseGitLocation(loc string) (string, string, string, string, string, string, error) {
	isShort := false
	var repoPath, repoName, libName, repoURL, pathWithinRepo string
	u, err := url.Parse(loc)
	if err != nil {
		if sm := gitSSHShortRegex.FindAllStringSubmatch(loc, 1); sm != nil {
			u = &url.URL{
				User: url.User(sm[0][1]),
				Host: sm[0][2],
				Path: sm[0][3],
			}
			isShort = true
		} else {
			return "", "", "", "", "", "", errors.Errorf("%q is not a Git location spec 1", loc)
		}
	} else if u.Host == "" && u.Opaque != "" {
		/* Git short-hand repo path w/o user@, i.e. server:name */
		u = &url.URL{
			Host: u.Scheme,
			Path: u.Opaque,
		}
		isShort = true
	} else if u.Scheme == "" {
		return "", "", "", "", "", "", errors.Errorf("%q is not a Git location spec 2", loc)
	}

	parts := strings.Split(u.Path, "/")
	if len(parts) == 0 {
		return "", "", "", "", "", "", errors.Errorf("path is empty in %q", loc)
	}
	libName = parts[len(parts)-1]
	if strings.HasSuffix(libName, ".git") {
		libName = libName[:len(libName)-4]
	}
	// Now find where repo name ends and path within repo begins.
	if u.Scheme == "https" {
		// For GitHub HTTP URLs we expect {repoPath}/tree/{branch}/{pathWithinRepo}
		if u.Host == "github.com" {
			if len(parts) > 4 {
				repoPath = strings.Join(parts[1:3], "/")
				repoName = parts[2]
				u.Path = strings.Join(parts[:3], "/")
				pathWithinRepo = filepath.Join(parts[5:]...)
			}
		} else {
			// GitLab HTTP URLs have {repoPath}/-/tree/{branch}/{pathWithinRepo}
			for i, p := range parts {
				if i > 2 && p == "tree" && parts[i-1] == "-" && i < len(parts)-2 {
					repoPath = strings.Join(parts[1:i-1], "/")
					repoName = parts[i-2]
					u.Path = strings.Join(parts[:i-1], "/")
					pathWithinRepo = filepath.Join(parts[i+2:]...)
				}
			}
		}
	}
	// For everything else we look for the first component that ends in ".git".
	if repoName == "" {
		pathParts := []string{}
		for i, part := range parts {
			if strings.HasSuffix(part, ".git") {
				repoName = part[:len(part)-4]
				if i+1 < len(parts) {
					u.Path = strings.Join(parts[:i+1], "/")
					pathWithinRepo = filepath.Join(parts[i+1:]...)
				}
				pathParts = append(pathParts, repoName)
				break
			} else {
				if part != "" {
					repoName = part
					pathParts = append(pathParts, repoName)
				}
			}
		}
		repoPath = strings.Join(pathParts, "/")
	}

	if isShort {
		if u.User != nil && u.User.Username() != "" {
			repoURL = fmt.Sprintf("%s@%s:%s", u.User.Username(), u.Host, u.Path)
		} else {
			repoURL = fmt.Sprintf("%s:%s", u.Host, u.Path)
		}
	} else {
		repoURL = u.String()
	}

	return u.Host, repoPath, repoName, libName, repoURL, pathWithinRepo, nil
}

func (m *SWModule) Normalize() error {
	if m.Location == "" && m.OriginOld != "" {
		m.Location = m.OriginOld
	}
	if m.Location == "" {
		return fmt.Errorf("location is not set")
	}
	m.OriginOld = ""
	if m.Name == "" {
		n, err := m.GetName()
		if err != nil {
			return errors.Annotatef(err, "name of lib is not set and cannot be guessed from %q", m.Location)
		}
		m.Name = n
	}
	return nil
}

// PrepareLocalDir prepares local directory, if that preparation is needed
// in the first place, and returns the path to it. If defaultVersion is an
// empty string or "latest", then the default will depend on the kind of lib
// (e.g. for git it's "master")
func (m *SWModule) PrepareLocalDir(
	libsDir string, logWriter io.Writer, deleteIfFailed bool, defaultVersion string,
	pullInterval time.Duration, cloneDepth int,
) (string, error) {
	if m.localPath != "" {
		return m.localPath, nil
	}

	var err error
	localPath, repoVersion, isDirty := "", "", false
	switch m.GetType() {
	case SWModuleTypeGit:
		localRepoPath, err := m.getLocalGitRepoDir(libsDir, defaultVersion)
		if err != nil {
			return "", errors.Trace(err)
		}
		n, err := m.GetName()
		if err != nil {
			return "", errors.Trace(err)
		}
		_, _, _, _, repoURL, pathWithinRepo, err := parseGitLocation(m.Location)
		version := m.getVersionGit(defaultVersion)
		if repoVersion, isDirty, err = prepareLocalCopyGit(n, repoURL, version, localRepoPath, logWriter, deleteIfFailed, pullInterval, cloneDepth, m.credentials); err != nil {
			return "", errors.Trace(err)
		}

		if pathWithinRepo != "" {
			localPath = filepath.Join(localRepoPath, pathWithinRepo)
			st, err := os.Stat(localPath)
			if err != nil {
				return "", errors.Errorf("%q does not exist within %q", pathWithinRepo, localRepoPath)
			}
			if !st.IsDir() {
				return "", errors.Errorf("%q is not a directory", localPath)
			}
		} else {
			localPath = localRepoPath
		}

	case SWModuleTypeLocal:
		localPath, err = m.GetLocalDir(libsDir, defaultVersion)
		if err != nil {
			return "", errors.Trace(err)
		}
	}

	// Everything went fine, so remember local path (and return it later)
	m.localPath = localPath
	m.repoVersion = repoVersion
	m.isDirty = isDirty

	return localPath, nil
}

func (m *SWModule) FetchPrebuiltBinary(platform, defaultVersion, tgt string) error {
	version := m.GetVersion(defaultVersion)
	switch m.GetType() {
	case SWModuleTypeGit:
		repoHost, repoPath, _, libName, _, _, err := parseGitLocation(m.Location)
		if err != nil {
			return errors.Trace(err)
		}
		assetName := fmt.Sprintf("lib%s-%s.a", libName, platform)
		assetAPIType := m.AssetAPI
		if assetAPIType == "" {
			switch {
			case strings.Contains(m.Location, "github"):
				assetAPIType = AssetAPIGitHub
			case strings.Contains(m.Location, "gitlab"):
				assetAPIType = AssetAPIGitLab
			default:
				return errors.Annotatef(err, "%s: asset_api not specified and could not be guessed", libName)
			}
		}
		var assetData []byte
		switch assetAPIType {
		case AssetAPIGitHub:
			token := ""
			if m.credentials != nil {
				token = m.credentials.Pass
			}
			for i := 1; i <= 3; i++ {
				assetData, err = fetchGitHubAsset(m.Location, repoHost, repoPath, version, assetName, token)
				if err == nil || os.IsNotExist(errors.Cause(err)) {
					break
				}
				// Sometimes asset downloads fail. GitHub doesn't like us, or rate limiting or whatever.
				// Try a couple times.
				glog.Errorf("GitHub asset %s download failed (attempt %d): %s", assetName, i, err)
				time.Sleep(1 * time.Second)
			}
		case AssetAPIGitLab:
			token := ""
			if m.credentials != nil {
				token = m.credentials.Pass
			}
			assetData, err = fetchGitLabAsset(repoHost, repoPath, version, assetName, token)
		}
		if err != nil {
			return errors.Annotatef(err, "%s: failed to download %s asset %s", libName, assetAPIType, assetName)
		}

		if err := os.MkdirAll(filepath.Dir(tgt), 0755); err != nil {
			return errors.Trace(err)
		}

		if err := ioutil.WriteFile(tgt, assetData, 0644); err != nil {
			return errors.Trace(err)
		}
		return nil
	}

	name, _ := m.GetName()
	return errors.Errorf("unable to fetch prebuilt binary for %q", name)
}

func (m *SWModule) GetVersion(defaultVersion string) string {
	version := m.Version
	if version == "" {
		version = defaultVersion
	}
	return version
}

func (m *SWModule) SetVersionOverride(version string) {
	m.versionOverride = version
}

func (m *SWModule) getVersionGit(defaultVersion string) string {
	version := m.versionOverride
	if version == "" {
		version = m.GetVersion(defaultVersion)
	}
	if version == "latest" {
		version = "master"
	}
	return version
}

func (m *SWModule) getLocalGitRepoDir(libsDir, defaultVersion string) (string, error) {
	if m.GetType() != SWModuleTypeGit {
		return "", errors.Errorf("%q is not a Git lib", m.Location)
	}
	_, _, repoName, _, _, _, err := parseGitLocation(m.Location)
	if err != nil {
		return "", errors.Trace(err)
	}
	return filepath.Join(libsDir, repoName), nil
}

func (m *SWModule) GetLocalDir(libsDir, defaultVersion string) (string, error) {
	switch m.GetType() {
	case SWModuleTypeGit:
		localRepoPath, err := m.getLocalGitRepoDir(libsDir, defaultVersion)
		if err != nil {
			return "", errors.Trace(err)
		}
		_, _, _, _, _, pathWithinRepo, _ := parseGitLocation(m.Location)
		return filepath.Join(localRepoPath, pathWithinRepo), nil

	case SWModuleTypeLocal:
		if m.Location != "" {
			originAbs, err := filepath.Abs(m.Location)
			if err != nil {
				return "", errors.Trace(err)
			}

			return originAbs, nil
		} else if m.Name != "" {
			return filepath.Join(libsDir, m.Name), nil
		} else {
			return "", errors.Errorf("neither name nor location is specified")
		}

	default:
		return "", errors.Errorf("Illegal module type: %v", m.GetType())
	}
}

func (m *SWModule) GetRepoVersion() (string, bool, error) {
	if m.localPath == "" {
		return "", false, fmt.Errorf("directory has not been prepared yet")
	}
	return m.repoVersion, m.isDirty, nil
}

// For testing
func (m *SWModule) SetLocalPathAndRepoVersion(localPath, repoVersion string, isDirty bool) {
	m.localPath = localPath
	m.repoVersion = repoVersion
	m.isDirty = isDirty
}

// FetchableFromInternet returns whether the library could be fetched
// from the web
func (m *SWModule) FetchableFromWeb() (bool, error) {
	return false, nil
}

func (m *SWModule) GetName() (string, error) {
	n, err := m.getName()
	if err == nil && n == boardsLibName {
		n = boardsLibNewName
	}
	return n, err
}

func (m *SWModule) GetName2() (string, error) {
	n, err := m.GetName()
	if err == nil && n == boardsLibNewName {
		n = boardsLibName
	}
	return n, err
}

func (m *SWModule) getName() (string, error) {
	if m.Name != "" {
		if !validNameRegex.MatchString(m.Name) {
			return "", errors.Errorf("%q is not a valid name", m.Name)
		}
		return m.Name, nil
	}

	switch m.GetType() {
	case SWModuleTypeGit:
		_, _, _, libName, _, _, err := parseGitLocation(m.Location)
		return libName, err
	case SWModuleTypeLocal:
		_, name := filepath.Split(m.Location)
		if name == "" {
			return "", errors.Errorf("name is empty in the location %q", m.Location)
		}

		return name, nil
	default:
		return "", errors.Errorf("name is not specified, and the lib type is unknown")
	}
}

func (m *SWModule) GetHostName() string {
	switch m.GetType() {
	case SWModuleTypeGit:
		libHost, _, _, _, _, _, _ := parseGitLocation(m.Location)
		return libHost
	case SWModuleTypeLocal:
		return ""
	default:
		return ""
	}
}

func (m *SWModule) GetType() SWModuleType {
	stype := m.Type

	if m.Location == "" && m.Name == "" {
		return SWModuleTypeInvalid
	}

	if stype == "" {
		if m.Location != "" {
			u, err := url.Parse(m.Location)
			if err == nil {
				switch u.Scheme {
				case "ssh":
					stype = "git"
				case "https":
					switch u.Host {
					case "github.com":
						stype = "git"
					}
				default:
				}
			}
			if stype == "" && gitSSHShortRegex.MatchString(m.Location) {
				stype = "git"
			}
		}
	}

	switch stype {
	case "git":
		return SWModuleTypeGit
	default:
		return SWModuleTypeLocal
	}
}

func (m *SWModule) SetCredentials(creds *Credentials) {
	m.credentials = creds
}

func (m *SWModule) GetCredentials() *Credentials {
	return m.credentials
}

var (
	repoLocks     = map[string]*sync.Mutex{}
	repoLocksLock = sync.Mutex{}
)

func prepareLocalCopyGit(
	name, origin, version, targetDir string,
	logWriter io.Writer, deleteIfFailed bool,
	pullInterval time.Duration, cloneDepth int,
	creds *Credentials,
) (string, bool, error) {

	repoLocksLock.Lock()
	lock := repoLocks[targetDir]
	if lock == nil {
		lock = &sync.Mutex{}
		repoLocks[targetDir] = lock
	}
	repoLocksLock.Unlock()
	lock.Lock()
	defer lock.Unlock()
	return prepareLocalCopyGitLocked(name, origin, version, targetDir, logWriter, deleteIfFailed, pullInterval, cloneDepth, creds)
}

func prepareLocalCopyGitLocked(
	name, origin, version, targetDir string,
	logWriter io.Writer, deleteIfFailed bool,
	pullInterval time.Duration, cloneDepth int,
	creds *Credentials,
) (string, bool, error) {
	gitinst := mosgit.NewOurGit(BuildCredsToGitCreds(creds))
	// version is already converted from "" or "latest" to "master" here.

	// Check if we should clone or pull git repo inside of targetDir.
	// Valid cases are:
	//
	// - it does not exist: it will be cloned
	// - it exists, and is empty: it will be cloned
	// - it exists, and is a git repo: it will be pulled
	//
	// All other cases are considered as an error.
	repoExists := false
	if _, err := os.Stat(targetDir); err == nil {
		// targetDir exists; let's see if it's a git repo
		if _, err := os.Stat(filepath.Join(targetDir, ".git")); err == nil {
			// Yes it is a git repo
			repoExists = true
		} else {
			// No it's not a git repo; let's see if it's empty; if not, it's an error.
			files, err := ioutil.ReadDir(targetDir)
			if err != nil {
				return "", false, errors.Trace(err)
			}
			if len(files) > 0 {
				freportf(logWriter, "%q is not empty, but is not a git repository either, leaving it intact", targetDir)
				return "", false, nil
			}
		}
	} else if os.IsNotExist(err) {
		if pullInterval == 0 {
			return "", false, fmt.Errorf("%s: Local copy in %q does not exist and fetching is not allowed", name, targetDir)
		}
	} else {
		// Some error other than non-existing dir
		return "", false, errors.Trace(err)
	}

	if !repoExists {
		freportf(logWriter, "%s: Does not exist, cloning from %q...", name, origin)
		cloneOpts := ourgit.CloneOptions{
			Depth: cloneDepth,
		}
		// We specify the revision to clone if only depth is limited; otherwise,
		// we'll clone at master and checkout the needed revision afterwards,
		// because this use case is faster for go-git.
		if cloneDepth > 0 {
			cloneOpts.Ref = version
		}
		err := gitinst.Clone(origin, targetDir, cloneOpts)
		if err != nil {
			return "", false, errors.Trace(err)
		}
	} else {
		// Repo exists, let's check if the working dir is clean. If not, we'll
		// not do anything.
		isClean, err := mosgit.IsClean(gitinst, targetDir, version)
		if err != nil {
			return "", false, errors.Trace(err)
		}
		curHash, _ := gitinst.GetCurrentHash(targetDir)
		if !isClean {
			freportf(logWriter, "%s exists and is dirty, leaving it intact\n", targetDir)
			return curHash, true, nil
		}
	}

	// Now, we'll try to checkout the desired version.
	//
	// It's optimized for two common cases:
	// - We're already on the desired branch (in this case, pull will be performed)
	// - We're already on the desired tag (nothing will be performed)
	// - We're already on the desired SHA (nothing will be performed)
	//
	// All other cases will result in `git fetch`, which is much longer than
	// pull, but we don't care because it will happen if only we switch to
	// another version.

	// First of all, get current SHA
	curHash, err := gitinst.GetCurrentHash(targetDir)
	if err != nil {
		return "", false, errors.Trace(err)
	}

	glog.V(2).Infof("%s: Hash: %q", name, curHash)

	// Check if it's equal to the desired one
	if ourgit.HashesEqual(curHash, version) {
		glog.V(2).Infof("%s: hashes are equal %q, %q", name, curHash, version)
		// Desired version is a fixed SHA, and it's equal to the
		// current commit: we're all set.
		return curHash, false, nil
	}

	var branchExists, tagExists bool

	// Check if version is a known branch name
	branchExists, err = gitinst.DoesBranchExist(targetDir, version)
	if err != nil {
		return "", false, errors.Trace(err)
	}

	glog.V(2).Infof("%s: branch %q exists=%v", name, version, branchExists)

	// Check if version is a known tag name
	tagExists, err = gitinst.DoesTagExist(targetDir, version)
	if err != nil {
		return "", false, errors.Trace(err)
	}

	glog.V(2).Infof("%s: tag %q exists=%v", name, version, tagExists)

	// If the desired mongoose-os version isn't a known branch, do git fetch
	if !branchExists && !tagExists {
		glog.V(2).Infof("%s: %s is neither a branch nor a tag, fetching...", name, version)
		err = gitinst.Fetch(targetDir, version, ourgit.FetchOptions{Depth: 1})
		if err != nil {
			return "", false, errors.Trace(err)
		}

		// After fetching, refresh branchExists and tagExists
		branchExists, err = gitinst.DoesBranchExist(targetDir, version)
		if err != nil {
			return "", false, errors.Trace(err)
		}
		glog.V(2).Infof("%s: branch %q exists=%v", name, version, branchExists)

		// Check if version is a known tag name
		tagExists, err = gitinst.DoesTagExist(targetDir, version)
		if err != nil {
			return "", false, errors.Trace(err)
		}
		glog.V(2).Infof("%s: tag %q exists=%v", name, version, tagExists)
	}

	refType := ourgit.RefTypeHash
	if branchExists {
		glog.V(2).Infof("%s: %q is a branch", name, version)
		refType = ourgit.RefTypeBranch
	} else if tagExists {
		glog.V(2).Infof("%s: %q is a tag", name, version)
		refType = ourgit.RefTypeTag
	} else {
		// Given version is neither a branch nor a tag, let's see if it looks like
		// a hash
		if _, err := hex.DecodeString(version); err == nil {
			glog.V(2).Infof("%s: %q is neither a branch nor a tag, assume it's a hash", name, version)
		} else {
			return "", false, errors.Errorf("given version %q is neither a branch nor a tag", version)
		}
	}

	// Try to checkout to the requested version
	freportf(logWriter, "%s: Checking out %s...", name, version)
	err = gitinst.Checkout(targetDir, version, refType)
	if err != nil {
		return "", false, errors.Trace(err)
	}

	newHash, err := gitinst.GetCurrentHash(targetDir)
	if err != nil {
		return "", false, errors.Trace(err)
	}
	glog.V(2).Infof("%s: New hash: %s", name, newHash)

	if branchExists {

		// Pull the branch if we just switched to it (hash is different) or hasn't been pulled for pullInterval.
		wantPull := newHash != curHash

		if !wantPull && pullInterval != 0 {
			fInfo, err := os.Stat(targetDir)
			if err != nil {
				return "", false, errors.Trace(err)
			}
			if fInfo.ModTime().Add(pullInterval).Before(time.Now()) {
				wantPull = true
			}
		}

		if wantPull {
			freportf(logWriter, "%s: Pulling...", name)
			err = gitinst.Pull(targetDir, version)
			if err != nil {
				return "", false, errors.Trace(err)
			}

			// Update modification time
			if err := os.Chtimes(targetDir, time.Now(), time.Now()); err != nil {
				return "", false, errors.Trace(err)
			}
		} else {
			glog.Infof("Repository %q is recent enough, not updating", targetDir)
		}
	} else {
		glog.V(2).Infof("requested version %q is not a branch, skip pulling.", version)
	}

	// To be safe, do `git checkout .`, so that any possible corruptions
	// of the working directory will be fixed
	glog.V(2).Infof("resetting")
	err = gitinst.ResetHard(targetDir)
	if err != nil {
		return "", false, errors.Trace(err)
	}

	curHash, _ = gitinst.GetCurrentHash(targetDir)
	freportf(logWriter, "%s: Done, hash %s", name, curHash)

	return curHash, false, nil
}

func BuildCredsToGitCreds(creds *Credentials) *ourgit.Credentials {
	if creds == nil {
		return nil
	}
	return &ourgit.Credentials{User: creds.User, Pass: creds.Pass}
}

func freportf(logFile io.Writer, f string, args ...interface{}) {
	ourutil.Freportf(logFile, f, args...)
}
