#!/usr/bin/env python3
#
# Copyright (c) 2014-2018 Cesanta Software Limited
# All rights reserved
#
# Licensed under the Apache License, Version 2.0 (the ""License"");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an ""AS IS"" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import argparse
import multiprocessing
import os
import shutil
import subprocess
import tempfile

import git       # apt-get install python3-git || pip3 install GitPython
import requests  # apt-get install python3-requests || pip3 install requests

import github_api

token = ""

parser = argparse.ArgumentParser(description='')
parser.add_argument('--release-tag', required=True, default = "", help = 'Release tag, like 1.12')
parser.add_argument('--token_filepath', type=str, default='/secrets/github/cesantabot/github_token')
parser.add_argument('--tmpdir', type=str, default=os.path.expanduser('~/mos_release_tmp'))
parser.add_argument('--parallelism', type=int, default=32)
parser.add_argument('repo', type=str, nargs='*', help='Run on specific repos. If not specified, runs on all repos.')

args = parser.parse_args()

if not os.path.isfile(args.token_filepath):
    print("++ Token file %s does not exist, exiting" % args.token_filepath)
    exit(1)

with open(args.token_filepath, 'r') as f:
    token = f.read().strip()

def call_users_api(
        org, users_url, params = {}, method = "GET", json_data = None, subdomain = "api",
        data = None, headers = {}, decode_json = True
        ):
    return github_api.call_users_api(token=token, **locals())

def call_releases_api(
        repo_name, releases_url, params = {}, method = "GET", json_data = None, subdomain = "api",
        data = None, headers = {}, decode_json = True
        ):
    return github_api.call_releases_api(token=token, **locals())


def get_repos(org):
    repos = []
    page = 1
    while True:
        # Get repos on the current "page"
        r, ok = call_users_api(org, "/repos", params={"page": page})

        if len(r) == 0:
            # No more repos, we're done
            break

        repos += r
        page += 1

    return repos


def get_repo_names(org):
    repo_names = []
    for repo in get_repos(org):
        repo_names.append(repo["full_name"])
    return repo_names


def make_repo_tag(repo_name, tag):
    local_path = os.path.join(args.tmpdir, repo_name)
    if os.path.isdir(local_path):
        shutil.rmtree(local_path)
    print("%s: Cloning to %s" % (repo_name, local_path))
    repo = git.Repo.clone_from("git@github.com:%s" % repo_name, local_path)

    force = False
    for t in repo.tags:
        if t.name == tag:
            print("%s: Deleting tag %s => %s" % (repo_name, tag, t.commit))
            repo.delete_tag(tag)
            force = True
            break
    print("%s: Creating and pushing tag %s" % (repo_name, tag))
    nt = repo.create_tag(tag)
    repo.remotes.origin.push(nt, force=force)
    print("%s: Pushed, %s => %s" % (repo_name, tag, nt.commit))


def del_repo_tag(repo_name, tag):
    local_path = os.path.join(args.tmpdir, repo_name)
    if os.path.isdir(local_path):
        shutil.rmtree(local_path)
    print("Cloning %s to %s" % (repo_name, local_path))
    repo = git.Repo.clone_from("git@github.com:%s" % repo_name, local_path)

    # TODO(dfrank): delete tag only if it exists. It's surprisingly hard to
    # check whether the tag exists.
    try:
        repo.delete_tag(tag)
    except:
        pass

    print("Deleting tag %s on %s" % (tag, repo_name))
    repo.remotes.origin.push(":%s" % tag)
    print("Deleted on %s" % repo_name)


def del_release(repo_name, tag):
    r, ok = call_releases_api(repo_name, releases_url = "/tags/%s" % tag)
    if ok:
        print("%s: Deleting existing %s release %d" % (repo_name, tag, r["id"]))
        r, ok = call_releases_api(
            repo_name,
            releases_url = "/%d" % r["id"],
            method = "DELETE",
            decode_json = False
            )

# handle_repo {{{
def handle_repo(repo_name, from_tag, to_tag):
    # Delete existing release, if any.
    del_release(repo_name, to_tag)

    # Tag the repo (deletes existing tag, if any).
    make_repo_tag(repo_name, to_tag)

    res, ok = call_releases_api(repo_name, "/tags/%s" % from_tag)
    if not ok:
        print("%s: No %s release, not creating tagged" % (repo_name, from_tag))
        # No release - no problem.
        return

    # Create a release draft {{{
    print("%s: Creating a new draft of %s" % (repo_name, to_tag))
    r, ok = call_releases_api(repo_name, method = "POST", releases_url = "", json_data = {
        "tag_name": to_tag,
        "name": to_tag,
        "draft": True,
    })
    if not ok:
        raise Exception("Failed to create a draft: %s" % r)

    new_rel_id = r["id"]
    # }}}

    for asset in res["assets"]:
        asset_url = asset["browser_download_url"]
        print("%s: Downloading %s" % (repo_name, asset_url))
        r = requests.get(asset_url)
        if r.status_code == 200:

            print("%s: Uploading a new asset %s" % (repo_name, asset["name"]))
            r, ok = call_releases_api(
                repo_name,
                method = "POST", subdomain = "uploads", data = r.content,
                releases_url = "/%d/assets" % new_rel_id,
                headers = {
                    "Content-Type": asset["content_type"]
                },
                params = {
                    "name": asset["name"]
                }
              )
            if not ok:
                raise Exception("Failed to upload %s: %s" % (asset["name"], r))

        else:
            raise Exception("Failed to download asset: %s" % r.text)

    # Undraft the release {{{
    print("%s: Undraft the %s release" % (repo_name, to_tag))
    r, ok = call_releases_api(repo_name, method = "PATCH", releases_url = "/%d" % new_rel_id, json_data = {
        "draft": False,
    })
    if not ok:
        raise Exception("Failed to undraft the release: %s" % r)
    # }}}
# }}}

# Wrappers which return an error instead of throwing it, this is for
# them to work in multithreading.pool
def handle_repo_noexc(repo_name, from_tag, to_tag):
    try:
        handle_repo(repo_name, from_tag, to_tag)
        return None
    except Exception as e:
        return (repo_name, str(e))

repo_root = os.path.realpath(os.path.join(__file__, "..", ".."))

repos = args.repo

if not repos:
    # Get libs and apps repos
    repos = get_repo_names("mongoose-os-libs") + get_repo_names("mongoose-os-apps")
    # Add a few more
    repos.extend(["cesanta/mongoose-os", "cesanta/mjs", "cesanta/mos-libs", "cesanta/mos-tool"])

pool = multiprocessing.Pool(processes=args.parallelism)
results = []

# Enqueue repos which need a release to be copied, with all assets etc
for repo_name in repos:
    results.append((repo_name, pool.apply_async(handle_repo_noexc, [repo_name, "latest", args.release_tag])))

# Wait for all tasks to complete, and collect errors {{{
errs = []
for res in results:
    result = res[1].get()
    if result != None:
        errs.append(result)
# }}}

if len(errs) == 0:
    print("Success!")
else:
    print("------------------------------------------------------")
    print("Errors: %d" % len(errs))
    for err in errs: # Replace `None` as you need.
        print("ERROR in %s: %s" % (err[0], err[1]))
        print("---")
    exit(1)
