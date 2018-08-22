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
import hashlib
import os
import logging
import re
import shutil
import subprocess
import sys
import tempfile

import requests  # apt-get install python3-requests || pip3 install requests

BEGIN_MARKER = "update_hb begin"
END_MARKER = "update_hb end"


def CloneHBRepo(hb_repo, hb_repo_dir):
    logging.info("Cloning %s to %s...", hb_repo, hb_repo_dir)
    subprocess.check_call(["git", "clone", "--depth=1", hb_repo, hb_repo_dir])


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--hb-repo", required=True)
    parser.add_argument("--formula", required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--blob-url", required=True)
    parser.add_argument("--blob-sha256", default="")
    parser.add_argument("--push", action="store_true")
    args = parser.parse_args()
    logging.basicConfig(level=logging.INFO, format="[%(asctime)s %(levelno)d] %(message)s", datefmt="%Y/%m/%d %H:%M:%S")
    blob_sha256 = args.blob_sha256
    if not blob_sha256:
        logging.info("Computing SHA256 of %s...", args.blob_url)
        r = requests.get(args.blob_url, allow_redirects=True)
        if r.status_code != 200:
            logging.error("Failed to fetch %s", args.blob_url)
            exit(1)
        blob_sha256 = hashlib.sha256(r.content).hexdigest()
        logging.info("SHA256: %s (%d bytes)", blob_sha256, len(r.content))

    with tempfile.TemporaryDirectory() as td:
        hb_repo_dir = os.path.join(td, args.hb_repo.split("/")[-1].replace(".git", ""))
        CloneHBRepo(args.hb_repo, hb_repo_dir)
        os.chdir(hb_repo_dir)
        recipe_file = os.path.join(hb_repo_dir, "Formula", "%s.rb" % args.formula)
        logging.info("Editing %s...", recipe_file)
        with open(recipe_file, "r") as rf:
            lines = rf.readlines()
        started = False
        for i, l in list(enumerate(lines)):
            if BEGIN_MARKER in l:
                started = True
            elif END_MARKER in l:
                started = False
            elif started:
                parts = l.strip().split()
                if len(parts) > 1:
                    if parts[0] == "url":
                        lines[i] = '  url "%s"\n' % args.blob_url
                    elif parts[0] == "sha256":
                        lines[i] = '  sha256 "%s"\n' % blob_sha256
                    elif parts[0] == "version":
                        lines[i] = '  version "%s"\n' % args.version
        with open(recipe_file, "w") as rf:
            rf.write("".join(lines))
        subprocess.check_call(["git", "--no-pager", "diff"])
        subprocess.check_call(["git", "commit", "-a",  "-m", "update_hb: %s %s" % (args.formula, args.version)])
        if args.push:
            logging.info("Pushing...")
            subprocess.check_call(["git", "push", "origin", "master"])
        else:
            logging.info("This is a dry run, not pushing. You can examine %s now.", hb_repo_dir)
            sys.stdin.readline()
