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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/cli/mosgit"
	"github.com/mongoose-os/mos/cli/ourutil"
	"github.com/mongoose-os/mos/common/ourgit"

	"github.com/golang/glog"
	"github.com/juju/errors"
)

type SWModule struct {
	Type string `yaml:"type,omitempty" json:"type,omitempty"`
	// Origin is deprecated since 2017/08/18
	OriginOld string `yaml:"origin,omitempty" json:"origin,omitempty"`
	Location  string `yaml:"location,omitempty" json:"location,omitempty"`
	Version   string `yaml:"version,omitempty" json:"version,omitempty"`
	Name      string `yaml:"name,omitempty" json:"name,omitempty"`
	Variant   string `yaml:"variant,omitempty" json:"variant,omitempty"`

	localPath string
}

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

func parseGitLocation(loc string) (string, string, string, string, string, error) {
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
			return "", "", "", "", "", errors.Errorf("%q is not a Git location spec 1", loc)
		}
	} else if u.Host == "" && u.Opaque != "" {
		/* Git short-hand repo path w/o user@, i.e. server:name */
		u = &url.URL{
			Host: u.Scheme,
			Path: u.Opaque,
		}
		isShort = true
	} else if u.Scheme == "" {
		return "", "", "", "", "", errors.Errorf("%q is not a Git location spec 2", loc)
	}

	parts := strings.Split(u.Path, "/")
	if len(parts) == 0 {
		return "", "", "", "", "", errors.Errorf("path is empty in %q", loc)
	}
	libName = parts[len(parts)-1]
	if strings.HasSuffix(libName, ".git") {
		libName = libName[:len(libName)-4]
	}
	// Now find where repo name ends and path within repo begins.
	// For GitHub HTTP URLs we expect tree/*/, for all other we find first component that ends in ".git".
	if u.Scheme == "https" && u.Host == "github.com" && len(parts) > 4 {
		repoPath = strings.Join(parts[1:3], "/")
		repoName = parts[2]
		u.Path = strings.Join(parts[:3], "/")
		pathWithinRepo = filepath.Join(parts[5:]...)
	} else {
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

	return repoPath, repoName, libName, repoURL, pathWithinRepo, nil
}

func (m *SWModule) Normalize() error {
	if m.Location == "" && m.OriginOld != "" {
		m.Location = m.OriginOld
	} else {
		// Just for the compatibility with a bit older fwbuild
		m.OriginOld = m.Location
	}
	if m.Name == "" {
		n, err := m.GetName()
		if err != nil {
			return errors.Annotatef(err, "name of lib is not set and cannot be guessed from %q", m.Location)
		}
		m.Name = n
	}
	return nil
}

// IsClean returns whether the local library repo is clean. Non-existing
// dir is considered clean.
func (m *SWModule) IsClean(libsDir, defaultVersion string) (bool, error) {
	switch m.GetType() {
	case SWModuleTypeGit:
		rp, err := m.getLocalGitRepoDir(libsDir, defaultVersion)
		if err != nil {
			return false, errors.Trace(err)
		}
		if _, err := os.Stat(rp); err != nil {
			if os.IsNotExist(err) {
				// Dir does not exist: we treat it as "dirty", just in order to fetch
				// all libs locally, so that it's more obvious for people that they can
				// edit those libs
				return false, nil
			}

			// Some error other than non-existing dir
			return false, errors.Trace(err)
		}

		// Dir exists, check if the repo is clean
		gitinst := mosgit.NewOurGit()
		isClean, err := mosgit.IsClean(gitinst, rp, m.getVersionGit(defaultVersion))
		if err != nil {
			return false, errors.Trace(err)
		}
		return isClean, nil
	case SWModuleTypeLocal:
		// Local libs can't be "clean", because there's no way for remote builder
		// to get them on its own
		return false, nil
	default:
		return false, errors.Errorf("wrong type: %v", m.GetType())
	}

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
	localPath := ""
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
		_, _, _, repoURL, pathWithinRepo, err := parseGitLocation(m.Location)
		version := m.getVersionGit(defaultVersion)
		if err = prepareLocalCopyGit(n, repoURL, version, localRepoPath, logWriter, deleteIfFailed, pullInterval, cloneDepth); err != nil {
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

	return localPath, nil
}

func getGHToken(tokenStr string) (string, error) {
	if tokenStr == "" {
		return "", nil
	}
	token := ""
	if len(tokenStr) > 1 && tokenStr[0] == '@' {
		if tokenData, err := ioutil.ReadFile(tokenStr[1:]); err != nil {
			return "", errors.Trace(err)
		} else {
			token = string(tokenData)
		}
	} else {
		token = tokenStr
	}
	return strings.TrimSpace(token), nil
}

func fetchGitHubAsset(loc, owner, repo, tag, assetName string) ([]byte, error) {
	// Try public URL first. Most of our repos (and therefore assets) are public.
	// API access limits do not apply to public asset access.
	if strings.HasPrefix(loc, "https://") {
		assetURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", owner, repo, tag, assetName)
		data, err := fetchGitHubAssetFromURL(assetName, tag, assetURL)
		if err == nil {
			return data, nil
		}
	}
	relMetaURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", owner, repo, tag)
	client := &http.Client{}
	req, err := http.NewRequest("GET", relMetaURL, nil)
	token, err := getGHToken(*flags.GHToken)
	if err != nil {
		return nil, errors.Annotatef(err, "invalid --gh-token")
	}
	if token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("token %s", token))
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Annotatef(err, "failed to fetch %s", relMetaURL)
	}
	defer resp.Body.Close()
	assetURL := ""
	if resp.StatusCode == http.StatusOK {
		relMetaData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Trace(err)
		}
		glog.V(4).Infof("%s/%s/%s/%s: Release metadata: %s", owner, repo, tag, assetName, string(relMetaData))
		var relMeta struct {
			ID     int `json:"id"`
			Assets []*struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			}
		}
		if err = json.Unmarshal(relMetaData, &relMeta); err != nil {
			return nil, errors.Annotatef(err, "failed to parse GitHub release info")
		}
		for _, a := range relMeta.Assets {
			if a.Name == assetName {
				assetURL = a.URL
				break
			}
		}
		if assetURL == "" {
			return nil, errors.Annotatef(os.ErrNotExist, "%s/%s: no asset %s found in release %s", owner, repo, assetName, tag)
		}
	} else {
		return nil, errors.Errorf("got %d status code when fetching %s (note: private repos may need --gh-token)", resp.StatusCode, relMetaURL)
	}
	glog.Infof("%s/%s/%s/%s: Asset URL: %s", owner, repo, tag, assetName, assetURL)
	return fetchGitHubAssetFromURL(assetName, tag, assetURL)
}

func fetchGitHubAssetFromURL(assetName, tag, assetURL string) ([]byte, error) {
	ourutil.Reportf("Fetching %s (%s) from %s...", assetName, tag, assetURL)

	client := &http.Client{}
	req, err := http.NewRequest("GET", assetURL, nil)
	req.Header.Add("Accept", "application/octet-stream")
	token, err := getGHToken(*flags.GHToken)
	if err != nil {
		return nil, errors.Annotatef(err, "invalid --gh-token")
	}
	if token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("token %s", token))
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Annotatef(err, "failed to fetch %s", assetURL)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("got %d status code when fetching %s", resp.StatusCode, assetURL)
	}
	// Fetched the asset successfully
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return data, nil
}

func (m *SWModule) FetchPrebuiltBinary(platform, defaultVersion, tgt string) error {
	version := m.GetVersion(defaultVersion)
	switch m.GetType() {
	case SWModuleTypeGit:
		if !strings.Contains(m.Location, "github.com") {
			break
		}
		repoPath, _, libName, _, _, err := parseGitLocation(m.Location)
		if err != nil {
			return errors.Trace(err)
		}
		pp := strings.Split(repoPath, "/")
		assetName := fmt.Sprintf("lib%s-%s.a", libName, platform)
		var data []byte
		for i := 1; i <= 3; i++ {
			data, err = fetchGitHubAsset(m.Location, pp[0], pp[1], version, assetName)
			if err == nil || os.IsNotExist(errors.Cause(err)) {
				break
			}
			// Sometimes asset downloads fail. GitHub doesn't like us, or rate limiting or whatever.
			// Try a couple times.
			glog.Errorf("GitHub asset %s download failed (attempt %d): %s", assetName, i, err)
			time.Sleep(1 * time.Second)
		}
		if err != nil {
			return errors.Annotatef(err, "failed to download %s", assetName)
		}

		if err := os.MkdirAll(filepath.Dir(tgt), 0755); err != nil {
			return errors.Trace(err)
		}

		if err := ioutil.WriteFile(tgt, data, 0644); err != nil {
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

func (m *SWModule) getVersionGit(defaultVersion string) string {
	version := m.GetVersion(defaultVersion)
	if version == "latest" {
		version = "master"
	}
	return version
}

func (m *SWModule) getLocalGitRepoDir(libsDir, defaultVersion string) (string, error) {
	if m.GetType() != SWModuleTypeGit {
		return "", errors.Errorf("%q is not a Git lib", m.Location)
	}
	_, repoName, _, _, _, err := parseGitLocation(m.Location)
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
		_, _, _, _, pathWithinRepo, _ := parseGitLocation(m.Location)
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
		_, _, libName, _, _, err := parseGitLocation(m.Location)
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

var (
	repoLocks     = map[string]*sync.Mutex{}
	repoLocksLock = sync.Mutex{}
)

func prepareLocalCopyGit(
	name, origin, version, targetDir string,
	logWriter io.Writer, deleteIfFailed bool,
	pullInterval time.Duration, cloneDepth int,
) (retErr error) {

	repoLocksLock.Lock()
	lock := repoLocks[targetDir]
	if lock == nil {
		lock = &sync.Mutex{}
		repoLocks[targetDir] = lock
	}
	repoLocksLock.Unlock()
	lock.Lock()
	defer lock.Unlock()
	return prepareLocalCopyGitLocked(name, origin, version, targetDir, logWriter, deleteIfFailed, pullInterval, cloneDepth)
}

func prepareLocalCopyGitLocked(
	name, origin, version, targetDir string,
	logWriter io.Writer, deleteIfFailed bool,
	pullInterval time.Duration, cloneDepth int,
) (retErr error) {
	gitinst := mosgit.NewOurGit()
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
				return errors.Trace(err)
			}
			if len(files) > 0 {
				freportf(logWriter, "%q is not empty, but is not a git repository either, leaving it intact", targetDir)
				return nil
			}
		}
	} else if !os.IsNotExist(err) {
		// Some error other than non-existing dir
		return errors.Trace(err)
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
			return errors.Trace(err)
		}
	} else {
		// Repo exists, let's check if the working dir is clean. If not, we'll
		// not do anything.
		isClean, err := mosgit.IsClean(gitinst, targetDir, version)
		if err != nil {
			return errors.Trace(err)
		}

		if !isClean {
			freportf(logWriter, "%s exists and is dirty, leaving it intact\n", targetDir)
			return nil
		}
	}

	// Now we know that the repo is either clean or non-existing, so, if asked to
	// delete in case of a failure, defer a fallback function.
	if deleteIfFailed {
		defer func() {
			if retErr != nil {
				// Instead of returning an error, try to delete the directory and
				// clone the fresh copy
				glog.Warningf("%s", retErr)
				glog.V(2).Infof("removing everything under %q", targetDir)

				files, err := ioutil.ReadDir(targetDir)
				if err != nil {
					glog.Errorf("failed to ReadDir(%q): %s", targetDir, err)
					return
				}
				for _, f := range files {
					glog.V(2).Infof("removing %q", f.Name())
					p := path.Join(targetDir, f.Name())
					if err := os.RemoveAll(p); err != nil {
						glog.Errorf("failed to remove %q: %s", p, err)
						return
					}
				}

				glog.V(2).Infof("calling prepareLocalCopyGit() again")
				retErr = prepareLocalCopyGitLocked(name, origin, version, targetDir, logWriter, false, pullInterval, cloneDepth)
			}
		}()
	}

	// Now, we'll try to checkout the desired mongoose-os version.
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
		return errors.Trace(err)
	}

	glog.V(2).Infof("%s: Hash: %q", name, curHash)

	// Check if it's equal to the desired one
	if ourgit.HashesEqual(curHash, version) {
		glog.V(2).Infof("%s: hashes are equal %q, %q", name, curHash, version)
		// Desired mongoose iot version is a fixed SHA, and it's equal to the
		// current commit: we're all set.
		return nil
	}

	var branchExists, tagExists bool

	// Check if MongooseOsVersion is a known branch name
	branchExists, err = gitinst.DoesBranchExist(targetDir, version)
	if err != nil {
		return errors.Trace(err)
	}

	glog.V(2).Infof("%s: branch %q exists=%v", name, version, branchExists)

	// Check if MongooseOsVersion is a known tag name
	tagExists, err = gitinst.DoesTagExist(targetDir, version)
	if err != nil {
		return errors.Trace(err)
	}

	glog.V(2).Infof("%s: tag %q exists=%v", name, version, tagExists)

	// If the desired mongoose-os version isn't a known branch, do git fetch
	if !branchExists && !tagExists {
		glog.V(2).Infof("%s: neither branch nor tag exists, fetching...", name)
		err = gitinst.Fetch(targetDir, ourgit.FetchOptions{})
		if err != nil {
			return errors.Trace(err)
		}

		// After fetching, refresh branchExists and tagExists
		branchExists, err = gitinst.DoesBranchExist(targetDir, version)
		if err != nil {
			return errors.Trace(err)
		}
		glog.V(2).Infof("%s: branch %q exists=%v", name, version, branchExists)

		// Check if version is a known tag name
		tagExists, err = gitinst.DoesTagExist(targetDir, version)
		if err != nil {
			return errors.Trace(err)
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
			return errors.Errorf("given version %q is neither a branch nor a tag", version)
		}
	}

	// Try to checkout to the requested version
	freportf(logWriter, "%s: Checking out %s...", name, version)
	err = gitinst.Checkout(targetDir, version, refType)
	if err != nil {
		return errors.Trace(err)
	}

	newHash, err := gitinst.GetCurrentHash(targetDir)
	if err != nil {
		return errors.Trace(err)
	}
	glog.V(2).Infof("%s: New hash: %s", name, newHash)

	if branchExists {

		// Pull the branch if we just switched to it (hash is different) or hasn't been pulled for pullInterval.
		wantPull := newHash != curHash

		if !wantPull && pullInterval != 0 {
			fInfo, err := os.Stat(targetDir)
			if err != nil {
				return errors.Trace(err)
			}
			if fInfo.ModTime().Add(pullInterval).Before(time.Now()) {
				wantPull = true
			}
		}

		if wantPull {
			freportf(logWriter, "%s: Pulling...", name)
			err = gitinst.Pull(targetDir)
			if err != nil {
				return errors.Trace(err)
			}

			// Update modification time
			if err := os.Chtimes(targetDir, time.Now(), time.Now()); err != nil {
				return errors.Trace(err)
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
		return errors.Trace(err)
	}

	curHash, _ = gitinst.GetCurrentHash(targetDir)
	freportf(logWriter, "%s: Done, hash %s", name, curHash)

	return nil
}

func freportf(logFile io.Writer, f string, args ...interface{}) {
	ourutil.Freportf(logFile, f, args...)
}
