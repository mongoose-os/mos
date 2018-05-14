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
parser.add_argument('tag', type=str, help='new tag')

parser.add_argument('--token_filepath', type=str, default='/secrets/github/cesantabot/github_token')
parser.add_argument('--tmpdir', type=str, default=os.path.expanduser('~/mos_release_tmp'))
parser.add_argument('--parallelism', type=int, default=32)

args = parser.parse_args()

# Read github token {{{
if not os.path.isfile(args.token_filepath):
    print("++ Token file %s does not exist, exiting" % args.token_filepath)
    exit(1)

with open(args.token_filepath, 'r') as f:
    token = f.read().strip()
# }}}

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
    print("Cloning %s to %s" % (repo_name, local_path))
    repo = git.Repo.clone_from("git@github.com:%s" % repo_name, local_path)

    # TODO(dfrank): delete tag only if it exists. It's surprisingly hard to
    # check whether the tag exists.
    try:
        repo.delete_tag(tag)
    except:
        pass

    print("Creating and pushing tag %s on %s" % (tag, repo_name))
    repo.create_tag(tag)
    repo.remotes.origin.push(tag)
    print("Done %s" % repo_name)

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

# del_release {{{
def del_release(repo_name, tag):
    r, ok = call_releases_api(repo_name, releases_url = "/tags/%s" % tag)
    if ok:
        print("++ Deleting existing %s release %d" % (tag, r["id"]))
        r, ok = call_releases_api(
            repo_name,
            releases_url = "/%d" % r["id"],
            method = "DELETE",
            decode_json = False
            )
# }}}

# copy_release {{{
def copy_release(repo_name, from_tag, to_tag):
    # If latest release already exists, delete it
    del_release(repo_name, to_tag)

    # Create a release draft {{{
    print("++ Creating a new draft of %s" % to_tag)
    r, ok = call_releases_api(repo_name, method = "POST", releases_url = "", json_data = {
        "tag_name": to_tag,
        "name": to_tag,
        "draft": True,
    })
    if not ok:
        raise Exception("Failed to create a draft: %s" % r)

    new_rel_id = r["id"]
    # }}}

    res, ok = call_releases_api(repo_name, "/tags/%s" % from_tag)
    if not ok:
        raise Exception("Failed to get existing tagged release: %s" % from_tag)

    for asset in res["assets"]:
        asset_url = asset["browser_download_url"]
        print("Downloading %s" % asset_url)
        r = requests.get(asset_url)
        if r.status_code == 200:

            print("uploading a new asset")
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
    print("++ Undraft the release")
    r, ok = call_releases_api(repo_name, method = "PATCH", releases_url = "/%d" % new_rel_id, json_data = {
        "draft": False,
    })
    if not ok:
        raise Exception("Failed to undraft the release: %s" % r)
    # }}}
# }}}

# Wrappers which return an error instead of throwing it, this is for
# them to work in multithreading.pool
def make_repo_tag_noexc(repo_name, tag):
    try:
        make_repo_tag(repo_name, tag)
        return None
    except Exception as e:
        return (repo_name, str(e))

def del_repo_tag_noexc(repo_name, tag):
    try:
        del_repo_tag(repo_name, tag)
        return None
    except Exception as e:
        return (repo_name, str(e))

def copy_release_noexc(repo_name, from_tag, to_tag):
    try:
        copy_release(repo_name, from_tag, to_tag)
        return None
    except Exception as e:
        return (repo_name, str(e))

def del_release_noexc(repo_name, tag):
    try:
        del_release(repo_name, tag)
        return None
    except Exception as e:
        return (repo_name, str(e))

repo_root = os.path.realpath(os.path.join(__file__, "..", ".."))

# Get repo names to copy a release: all libs and apps which we prebuild
# (and thus upload prebuilt binaries as github assets)
repo_names_for_release = []
apps = subprocess.check_output(
    ["make", "-s", "-C", "%s/mos_apps" % repo_root, "list_for_prebuilding"]
    ).decode("utf-8").split()
repo_names_for_release += ["mongoose-os-apps/" + s for s in apps]

libs = subprocess.check_output(
    ["make", "-s", "-C", "%s/mos_libs" % repo_root, "list_for_prebuilding"]
    ).decode("utf-8").split()
repo_names_for_release += ["mongoose-os-libs/" + s for s in libs]

# Get all apps and libs from github
repo_names_all = get_repo_names("mongoose-os-libs") + get_repo_names("mongoose-os-apps")

# Make sure repo_names_for_release only has repos actually present on github.
# E.g. we might have some non-published app which we don't really have to
# blacklist from prebuilding.
repo_names_for_release = list(set(repo_names_all) & set(repo_names_for_release))

# Get repo names to just tag: those which don't have a github release,
# plus cesanta/mongoose-os and cesanta/mjs
repo_names_for_tag = list(set(repo_names_all) - set(repo_names_for_release))
repo_names_for_tag.append("cesanta/mongoose-os")
repo_names_for_tag.append("cesanta/mjs")
repo_names_for_tag.append("cesanta/mos-libs")
repo_names_for_tag.append("cesanta/mos-tool")

pool = multiprocessing.Pool(processes=args.parallelism)
results = []

# Enqueue repos which need a release to be copied, with all assets etc
for repo_name in repo_names_for_release:
    results.append((repo_name, pool.apply_async(copy_release_noexc, [repo_name, "latest", args.tag])))

# Enqueue repos which need just a tag to be created

for repo_name in repo_names_for_tag:
    results.append((repo_name, pool.apply_async(make_repo_tag_noexc, [repo_name, args.tag])))

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
