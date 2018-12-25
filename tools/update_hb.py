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
    parser.add_argument("--hb-repo-dir")
    parser.add_argument("--formula", required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--blob-url", required=True)
    parser.add_argument("--blob-sha256", default="")
    parser.add_argument("--bottle", default=[], action="append")
    parser.add_argument("--bottle-upload-dest")
    parser.add_argument("--commit", action="store_true")
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

    bottles = {}
    for bottle_fname in args.bottle:
        bottle_base_name = os.path.basename(bottle_fname)
        # For some reason bottle name contains double dash, e.g.:
        #   mos--2.10.0.mojave.bottle.tar.gz
        # HB does not expect it when downloading, so adjust remote file name.
        if "--" in bottle_base_name:
            new_bottle_base_name = bottle_base_name.replace("--", "-")
            bottle_base_name = new_bottle_base_name
        bottle_fname = os.path.abspath(bottle_fname)
        with open(bottle_fname, "rb") as bf:
            sha256 = hashlib.sha256(bf.read()).hexdigest()
        parts = bottle_base_name.split(".")
        mac_os_version = parts[-4]
        bottles[mac_os_version] = (bottle_fname, bottle_base_name, sha256)

    hb_repo_dir = args.hb_repo_dir
    if not hb_repo_dir:
        td = tempfile.mkdtemp()
        hb_repo_dir = os.path.join(td, args.hb_repo.split("/")[-1].replace(".git", ""))

    if not os.path.exists(hb_repo_dir):
        CloneHBRepo(args.hb_repo, hb_repo_dir)

    os.chdir(hb_repo_dir)
    recipe_file = os.path.join(hb_repo_dir, "Formula", "%s.rb" % args.formula)
    logging.info("Editing %s...", recipe_file)
    with open(recipe_file, "r") as rf:
        lines = rf.readlines()

    new_lines = []
    started, in_bottle = False, False
    for i, l in list(enumerate(lines)):
        l = l.rstrip()
        copy = True
        if BEGIN_MARKER in l:
            started = True
        elif END_MARKER in l:
            started = False
        elif started:
            parts = l.strip().split()
            if not in_bottle:
                if len(parts) > 1:
                    if parts[0] == "url":
                        copy = False
                        new_lines.append('  url "%s"' % args.blob_url)
                    elif parts[0] == "sha256":
                        copy = False
                        new_lines.append('  sha256 "%s"' % blob_sha256)
                    elif parts[0] == "version":
                        copy = False
                        new_lines.append('  version "%s"' % args.version)
                    elif parts[0] == "bottle" and parts[1] == "do":
                        in_bottle = True
            else:
                if len(parts) > 1 and parts[0] == "sha256":
                    # Remove, we will re-generate the section.
                    copy = False
                elif len(parts) == 1 and parts[0] == "end":
                    in_bottle = False
                    for mac_os_version in sorted(bottles.keys()):
                        bottle_fname, bottle_base_name, sha256 = bottles[mac_os_version]
                        new_lines.append('    sha256 "%s" => :%s' % (sha256, mac_os_version))
                        if args.bottle_upload_dest:
                            upload_dst = "%s/%s" % (args.bottle_upload_dest, bottle_base_name)
                            print("Uploading %s to %s..." % (bottle_fname, upload_dst))
                            subprocess.check_call(["scp", bottle_fname, upload_dst])
        if copy:
            new_lines.append(l)

    with open(recipe_file, "w") as rf:
        rf.write("\n".join(new_lines))
        rf.write("\n")
    subprocess.check_call(["git", "--no-pager", "diff"])
    if args.commit:
        diff = subprocess.check_output(["git", "--no-pager", "diff"])
        if diff:
            subprocess.check_call([
                "git", "commit", "-a",
                "-m", "update_hb: %s %s" % (args.formula, args.version),
            ])
        else:
            print("Nothing changed")
    if args.push:
        logging.info("Pushing...")
        subprocess.check_call(["git", "push", "origin", "master"])
    else:
        logging.info("This is a dry run, not pushing. You can examine %s now.", hb_repo_dir)
