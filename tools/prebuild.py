#!/usr/bin/env python3
#
# A utility for prebuilding binary libs and apps.
# Can publish outputs to GitHub.
#
# Main input is a YAML config file that specifies which libs/apps to build
# in which variants and what to do with them.
#
# Top object of the config files is an array, each entry is as follows:
#  location: path to git repo
#  locations: [ multiple, paths, to, repos ]
#    location, locations or both can be used.
#  variants: array of variants, each variant must specify name and platform
#            and can additionally specify build vars and c/cxx flags.
#  out: output specification(s). currently only github output type is supported.
#       if output is not specified and input looks like a github repo,
#       github output is assumed.
#
# Example config file:
#
# - locations:
#    - https://github.com/mongoose-os-apps/demo-c
#    - https://github.com/mongoose-os-apps/demo-js
#   variants:
#     - name: esp8266
#       platform: esp8266
#     - name: esp8266-1M
#       platform: esp8266
#       build_vars:
#         FLASH_SIZE: 1048576
#
# This file will build 3 variants for each of the two apps and upload artifacts
# to GitHub.

import argparse
import copy
import github_api
import logging
import os
import shutil
import subprocess
import time
import yaml

# Debian/Ubuntu: apt-get install python-git
# PIP: pip install GitPython
import git

# NB: only support building from master right now.


g_cur_rel_id = None


def RunCmd(cmd):
    logging.info("  %s", " ".join(cmd))
    subprocess.check_call(cmd)


def CallReleasesAPI(
        repo_name, token, releases_url, params={}, method="GET", json_data=None, subdomain="api",
        data=None, headers={}, decode_json=True):
    return github_api.call_releases_api(**locals())


def DeleteRelease(repo, token, release_id):
    logging.info("    Deleting release %d", release_id)
    CallReleasesAPI(
        repo, token,
        method="DELETE",
        releases_url=("/%d" % release_id),
        decode_json=False)


def CreateGitHubRelease(spec, tag, token, tmp_dir):
    logging.debug("GH release spec: %s", spec)
    repo = spec["repo"]
    draft_rel_name = "%s_draft" % tag

    logging.info("Creating a release draft for %s", repo)
    r, ok = CallReleasesAPI(repo, token, "", method="POST", json_data={
        "name": tag,
        "draft": True,
        "tag_name": draft_rel_name,
        "target_commitish": "master",
    })
    if not ok:
        logging.error("Failed to create a release draft: %s", r)
        raise RuntimeError
    new_rel_id = r["id"]
    global g_cur_rel_id
    g_cur_rel_id = new_rel_id
    logging.debug("New release id: %d", new_rel_id)
    for asset_name, asset_file in spec["assets"]:
        ct = "application/zip" if asset_name.endswith(".zip") else "application/octet-stream"
        logging.info("  Uploading %s to %s", asset_file, asset_name)
        with open(asset_file, "rb") as f:
            i = 1
            while i <= 3:
                r, ok = CallReleasesAPI(
                    repo, token, method="POST", subdomain="uploads", data=f,
                    releases_url=("/%d/assets" % new_rel_id),
                    headers = {"Content-Type": ct},
                    params = {"name": asset_name})
                if ok:
                    break
                else:
                    logging.error("    Failed to upload %s (attempt %d): %s", asset_name, i, r)
                    time.sleep(1)
                    i += 1
            if not ok:
                logging.error("Failed to upload %s: %s", asset_name, r)
                raise RuntimeError

    # If target release already exists, delete it {{{
    r, ok = CallReleasesAPI(repo, token, releases_url=("/tags/%s" % tag))
    if ok:
        logging.info("  Deleting old %s release", tag)
        DeleteRelease(repo, token, r["id"])
        # Also remove the tag itself, so that when we save release below
        # with that tag, it'll be created on master branch.
        # Since we are not necessarily publishing from the same repo,
        # we make a separate clone.
        repo_url = "git@github.com:%s" % repo
        tmp_dst = os.path.join(tmp_dir, repo.replace("/", "_"))
        logging.info("  Deleting tag %s (tmp %s)", tag, tmp_dst)
        if not os.path.exists(tmp_dst):
            RunCmd(["git", "clone", "--depth=1", repo_url, tmp_dst])
        dst_repo = git.Repo(tmp_dst)
        remote = dst_repo.remote(name="origin")
        remote.push(refspec=(":%s" % tag))

    r, ok = CallReleasesAPI(
        repo, token, method="PATCH",
        releases_url=("/%d" % new_rel_id),
        json_data={
            "tag_name": tag,
            "draft": False,
            "target_commitish": "master",
        })
    if not ok:
        logging.error("Failed to create a release: %s", r)
        raise RuntimeError
    logging.info("Created release %s / %s (%d)", repo, tag, r["id"])
    g_cur_rel_id = None


def MakeAsset(an, asf, tmp_dir):
    af = os.path.join(tmp_dir, an)
    logging.info("  Copying %s -> %s", asf, af)
    shutil.copy(asf, af)
    return [an, af]


def ProcessLoc(e, loc, mos, tmp_dir, libs_dir, gh_release_tag, gh_token_file):
    parts = loc.split("/")
    pre, name, i, repo_loc, repo_subdir = "", "", 0, loc, ""
    for p in parts:
        pre = name.split(":")[-1]
        name = p
        i += 1
        if p.endswith(".git"):
            repo_loc = "/".join(parts[:i])
            repo_subdir = "/".join(parts[i:])
            name = p[:-4]
            break
    if repo_subdir:
        logging.info("== %s / %s", repo_loc, repo_subdir)
    else:
        logging.info("== %s", repo_loc)
    repo_dir = os.path.join(tmp_dir, pre, name)
    if os.path.exists(repo_loc):
        rl = repo_loc + ("/" if not repo_loc.endswith("/") else "")
        os.makedirs(repo_dir, exist_ok=True)
        cmd = ["rsync", "-a", "--delete", rl, repo_dir + "/"]
    else:
        if not os.path.exists(repo_dir):
            logging.info("Cloning into %s", repo_dir)
            cmd = ["git", "clone", repo_loc, repo_dir]
        else:
            logging.info("Pulling %s", repo_dir)
            cmd = ["git", "-C", repo_dir, "pull"]
    RunCmd(cmd)
    if repo_subdir:
        tgt_dir = os.path.join(repo_dir, repo_subdir)
    else:
        tgt_dir = repo_dir
    tgt_name = os.path.split(tgt_dir)[-1]
    assets = []
    common = e.get("common", {})
    # Build all the variants, collect assets
    for v in e["variants"]:
        logging.info(" %s %s", tgt_name, v["name"])
        os.chdir(tgt_dir)
        mos_cmd = [mos, "build", "--local", "--clean"]
        if libs_dir:
            mos_cmd.append("--libs-dir=%s" % libs_dir)
        mos_cmd.append("--platform=%s" % v["platform"])
        for bvk, bvv in sorted(list(common.get("build_vars", {}).items()) +
                               list(v.get("build_vars", {}).items())):
            mos_cmd.append("--build-var=%s=%s" % (bvk, bvv))
        cflags = (common.get("cflags", "") + " " + v.get("cflags", "")).strip()
        if cflags:
            mos_cmd.append("--cflags-extra=%s" % cflags)
        cxxflags = (common.get("cxxflags", "") + " " + v.get("cxxflags", "")).strip()
        if cflags:
            mos_cmd.append("--cxxflags-extra=%s" % cflags)
        mos_args = (common.get("mos_args", []) + v.get("mos_args", []))
        if mos_args:
            mos_cmd.extend(mos_args)
        RunCmd(mos_cmd)
        bl = os.path.join(tmp_dir, "%s-%s-build.log" % (tgt_name, v["name"]))
        logging.info("  Saving build log to %s", bl)
        shutil.copy(os.path.join(tgt_dir, "build", "build.log"), bl)
        # Ok, what did we just build?
        with open(os.path.join(tgt_dir, "mos.yml")) as f:
            m = yaml.load(f)
            if m.get("type", "") == "lib":
                assets.append(MakeAsset("lib%s-%s.a" % (tgt_name, v["name"]), os.path.join(tgt_dir, "build", "lib.a"), tmp_dir))
            else:
                assets.append(MakeAsset("%s-%s.zip" % (tgt_name, v["name"]), os.path.join(tgt_dir, "build", "fw.zip"), tmp_dir))
                assets.append(MakeAsset("%s-%s.elf" % (tgt_name, v["name"]), os.path.join(tgt_dir, "build", "objs", "fw.elf"), tmp_dir))
    outs = e.get("out", [])
    if not outs and loc.startswith("https://github.com/"):
        outs = [{"github": {"repo": "%s/%s" % (pre, tgt_name)}}]
    for out in outs:
        gh_out = copy.deepcopy(out.get("github", {}))
        # Push to GitHub
        if gh_out:
            gh_out["assets"] = assets
            gh_out["repo"] = gh_out["repo"] % {"repo_subdir": repo_subdir}

            if not gh_token_file:
                logging.info("Token file not set, GH uploads disabled")
                return
            if not os.path.isfile(gh_token_file):
                logging.error("Token file %s does not exist", gh_token_file)
                exit(1)
            logging.debug("Using token file at %s", gh_token_file)
            with open(gh_token_file, "r") as f:
                token = f.read().strip()

            try:
                CreateGitHubRelease(gh_out, gh_release_tag, token, tmp_dir)
            except (Exception, KeyboardInterrupt):
                if g_cur_rel_id:
                    DeleteRelease(gh_out["repo"], token, g_cur_rel_id)
                raise


def ProcessEntry(e, mos, tmp_dir, libs_dir, gh_release_tag, gh_token_file):
    for loc in e.get("locations", []) + [e.get("location")]:
        if loc: ProcessLoc(e, loc, mos, tmp_dir, libs_dir, gh_release_tag, gh_token_file)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Prebuild script for apps and libs")
    parser.add_argument("--v", type=int, default=logging.INFO)
    parser.add_argument("--config", type=str)
    parser.add_argument("--tmp-dir", type=str, default=os.path.join(os.getenv("TMPDIR", "/tmp"), "mos_prebuild"))
    parser.add_argument("--libs-dir", type=str)
    parser.add_argument("--mos", type=str, default="/usr/bin/mos")
    parser.add_argument("--gh-token-file", type=str)
    parser.add_argument("--gh-release-tag", type=str)
    args = parser.parse_args()

    logging.basicConfig(level=args.v, format="[%(asctime)s %(levelno)d] %(message)s", datefmt="%Y/%m/%d %H:%M:%S")
    logging.info("Reading %s", args.config)

    with open(args.config) as f:
        cfg = yaml.load(f)

    for e in cfg:
        ProcessEntry(e, args.mos, args.tmp_dir, args.libs_dir, args.gh_release_tag, args.gh_token_file)
    logging.info("All done")
