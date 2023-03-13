// Copyright (c) 2014-2017 Cesanta Software Limited
// All rights reserved

package ourgit

type OurGit interface {
	GetCurrentHash(localDir string) (string, error)
	DoesBranchExist(localDir string, branchName string) (bool, error)
	DoesTagExist(localDir string, tagName string) (bool, error)
	GetToplevelDir(localDir string) (string, error)
	Checkout(localDir string, id string, refType RefType) error
	ResetHard(localDir string) error
	Pull(localDir string, branch string) error
	Fetch(localDir string, what string, opts FetchOptions) error
	IsClean(localDir, version string, excludeGlobs []string) (bool, error)
	Clone(srcURL, localDir string, opts CloneOptions) error
	GetOriginURL(localDir string) (string, error)
}

type RefType string

type CloneOptions struct {
	// Path to a local repo which should be used as a reference for a new clone.
	// Equivalent of the --reference CLI flag.
	ReferenceDir string
	// How many commits to fetch. Equivalent of the --depth CLI flag.
	Depth int
	// Head to fetch: it can be a branch name, a tag name, or a hash.
	Ref string
}

type FetchOptions struct {
	// How many commits to fetch. Equivalent of the --depth CLI flag.
	Depth    int
	LocalRef string
}

const (
	RefTypeBranch RefType = "branch"
	RefTypeTag    RefType = "tag"
	RefTypeHash   RefType = "hash"

	minHashLen  = 6
	fullHashLen = 40
)

type Credentials struct {
	User string
	Pass string
}

func HashesEqual(hash1, hash2 string) bool {
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
