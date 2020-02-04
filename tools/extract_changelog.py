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
import sys
import tempfile

import github_api

token = ""

parser = argparse.ArgumentParser(description="")
parser.add_argument("--from", required=True, help = "Starting point (hash-like)")
parser.add_argument("--to", default="HEAD", help = "End point (hash-like)")
parser.add_argument("--no-pull", action="store_true", default=False,
                    help="Do not pull repos if exist, useful for repeated runs")
parser.add_argument("--tmpdir", type=str, default=os.path.expanduser("~/mos_release_tmp"))
parser.add_argument("--parallelism", type=int, default=32)
parser.add_argument("--token_filepath", type=str, default="/secrets/github/cesantabot/github_token")
parser.add_argument("repo", type=str, nargs="*", help="Run on specific repos. If not specified, runs on all repos.")

args = parser.parse_args()

TOKEN = "file:%s" % args.token_filepath

def get_repos(org):
    repos = []
    page = 1
    print("Listing repos under %s..." % org, file=sys.stderr)
    while True:
        # Get repos on the current "page"
        print("  %d..." % page, file=sys.stderr)
        r, ok = github_api.CallUsersAPI(org, TOKEN, "/repos", params={"page": page})

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


def handle_repo(repo_name, from_tag, to_tag):
    local_path = os.path.join(args.tmpdir, repo_name)
    if not os.path.isdir(local_path):
        print("%s: Cloning to %s" % (repo_name, local_path), file=sys.stderr)
        subprocess.check_call(["git", "clone", "git@github.com:%s" % repo_name, local_path])
    elif not args.no_pull:
        print("%s: Pulling %s" % (repo_name, local_path), file=sys.stderr)
        subprocess.check_call(["git", "-C", local_path, "pull", "-q"])

    cmd = ["git", "-C", local_path, "log", "--reverse", "--pretty=format:%H %ct%n%s%n%b%n---CUT---"]

    # See if "from" exists at all
    diff_res = subprocess.run(["git", "-C", local_path, "diff", from_tag], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    # If the starting point exists, restrict the range, otherwise
    # we assume taht the repo was created after from was created, so get the entire log.
    if diff_res.returncode == 0:
        cmd.append("%s...%s" % (from_tag, to_tag))

    log = subprocess.check_output(cmd)

    res = []

    ph, ch, ts, cl = 0, None, 0, ""
    for line in log.splitlines():
        line = str(line, "utf-8")
        if line == "---CUT---":
            if cl and cl != "none":
                res.append((ts, "[%s@%s](https://github.com/%s/commit/%s)" % (repo_name, ch[:7], repo_name, ch), cl))
            ph, ch, ts, cl = 0, None, 0, ""
            continue
        if ph == 0:
            ch, ts = line.split()
            ts = int(ts)
            ph = 1
            cl = ""
        elif ph == 1:
            if line.startswith("CL: "):
                cl = line[4:]
                ph = 2
            elif not cl:
                cl = line.strip()
        elif ph == 2:
            if line.startswith(" "):
                cl += "\n%s" % line.strip()
            elif line.startswith("CL: "):
                cl += "\n%s" % line[4:].strip()

    return res


def handle_repo_noexc(repo_name, from_tag, to_tag):
    try:
        return (repo_name, handle_repo(repo_name, from_tag, to_tag))
    except Exception as e:
        return (repo_name, str(e))

repos = args.repo

if not repos:
    # Get libs and apps repos
    repos = get_repo_names("mongoose-os-libs") + get_repo_names("mongoose-os-apps")
    # Add a few more
    repos.extend(["cesanta/mongoose-os", "cesanta/mjs", "mongoose-os/mos"])
    repos.sort()

    print("Repos: %s" % " ".join(repos), file=sys.stderr)

pool = multiprocessing.Pool(processes=args.parallelism)
results = []

for repo_name in repos:
    results.append((repo_name, pool.apply_async(handle_repo_noexc, [repo_name, getattr(args, "from"), args.to])))

# Wait for all tasks to complete, and collect errors
global_cl, errs = [], []
for res in results:
    repo, r = res[1].get()
    if type(r) is str:
        errs.append(r)
    else:
        for e in r:
            global_cl.append((repo,) + e)

# Dedup entries. When a single change affect multiple repos, we'll get dups.
cl_map = {}
for repo, ts, ch, cl in sorted(global_cl, key=lambda e: e[0]):
    oe = cl_map.get(cl)
    if oe:
        oe[3].append(ch)
    else:
        cl_map[cl] = (repo, ts, cl, [ch])

for e in sorted(cl_map.values(), key=lambda e: (e[0], e[1])):
    repo_short = e[0].split("/")[-1]
    print(" * %s: %s (%s)" % (repo_short, "\n   ".join(e[2].splitlines()), " ".join(e[3])))

if len(errs) != 0:
    print("------------------------------------------------------")
    print("Errors: %d" % len(errs))
    for err in errs: # Replace `None` as you need.
        print("ERROR in %s: %s" % (err[0], err[1]))
        print("---")
    exit(1)
