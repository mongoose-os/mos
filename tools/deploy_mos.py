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
import getpass
import json
import os
import pty
import re
import time
import select
import shutil
import subprocess
import sys

import git  # apt-get install python3-git || pip3 install GitPython

# Not used in this script but is used by other scripts.
# Importing now will catch missing dependencies early.
import github_api

GPG_KEY_PATH = os.path.join(os.environ["HOME"], ".gnupg-cesantabot")
BUILD_DEB_PATH = os.path.join("mos", "ubuntu", "build-deb.sh")
UPLOAD_DEB_PATH = os.path.join("mos", "ubuntu", "upload-deb.sh")
UBUNTU_VERSIONS = ["xenial", "bionic", "cosmic", "disco"]

deb_package = "mos-latest"
tag_effective = "latest"

def RunSubprocess(cmd, communicator=None, quiet=False):
    master_fd, slave_fd = pty.openpty()

    print("Running subprocess: %s" % " ".join(cmd))

    #-- run cmd as a child process, and pipe its stdin / stdout / stderr.
    proc = subprocess.Popen(
            cmd,
            stdin=slave_fd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE)

    out, out_line = b"", b""
    err_line = b""
    while not quiet:
        #-- get next byte from stdout
        ready, _, _ = select.select([proc.stdout, proc.stderr], [], [proc.stdout, proc.stderr])

        if proc.stdout in ready:
            byte = os.read(proc.stdout.fileno(), 1)
            if byte != b"":
                out += byte
                out_line += byte
                if byte == b"\n":
                    sys.stdout.write("[out] " + out_line.decode("utf-8"))
                    out_line = b""
                if communicator != None:
                    communicator(out_line, master_fd)
            else:
                sys.stdout.write("Child process exited\n")
                break
        if proc.stderr in ready:
            byte = os.read(proc.stderr.fileno(), 1)
            if byte != b"":
                err_line += byte
                if byte == b"\n":
                    sys.stdout.write("[err] " + err_line.decode("utf-8"))
                    err_line = b""

    proc.wait()
    if proc.returncode != 0:
        print("returncode: %d" % proc.returncode)
        print("stderr: %s" % proc.stderr.read())
        raise Exception("non-zero return code")

    return out


def UploaderComm(line, tty):
    if line == b"Enter passphrase: ":
        os.write(tty, bytes(passphrase + "\n", "utf-8"))
        sys.stdout.write("\n[sent passphrase]\n")
    elif line == b"Would you like to use the current signature? [Yn]":
        os.write(tty, bytes("y\n", "utf-8"))
        sys.stdout.write("\n[sent y]\n")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()

    parser.add_argument("--release-tag", default="", help="Release tag, like 1.12")
    parser.add_argument("--resume", type=int,  default=0, help="Resume from certain point")

    args = parser.parse_args()

    platform = "mac" if os.uname()[0] == "Darwin" else "linux"

    try:
        os.stat(BUILD_DEB_PATH)
        os.stat(UPLOAD_DEB_PATH)
    except Exception:
        print("This tool must be run from a mos-tool repo")
        exit(1)

    try:
        os.stat(GPG_KEY_PATH)
    except Exception:
        print("Package signing key (%s) does not exist. Go fetch it.")
        exit(1)

    # make sure we can run docker (and if not, fail early)
    print("Making sure Docker works...")
    RunSubprocess(["docker", "run", "--rm", "hello-world"], quiet=True)
    print("Ok, Docker works")

    # Check ssh access to site. If agent is used, this will unlock the key.
    print("Checking SSH access to the site...")
    RunSubprocess(["ssh", "core@mongoose-os.com", "echo", "Ok"])

    # Request the user for the passphrase
    passphrase = getpass.getpass("Passphrase for the key in %s: " % GPG_KEY_PATH)
    print("Checking GPG signing key passphrase...")
    RunSubprocess([
        "docker", "run", "-it", "--rm",
        "-v", "%s:/root/.gnupg" % GPG_KEY_PATH,
        "docker.io/mgos/ubuntu-golang:xenial",
        "gpg", "--sign", "--no-use-agent", "-o", "/dev/null", "/dev/null"],
        communicator=UploaderComm)
    print("Ok, passphrase is correct")

    if args.release_tag != "":
        deb_package = "mos"
        tag_effective = args.release_tag

        if args.resume == 0:
            # Make sure that the user didn't forget to stop publishing and make release tags.
            r = input("You made sure that publishing finished and stopped the timer, right? [y|N] ")
            if r != "y":
                print("I'm glad I asked. Go do that then.")
                exit(1)

            r = input("You ran 'tools/make_release_tags.py --release-tag %s' already, right? [y|N] " % tag_effective)
            if r != "y":
                print("I'm glad I asked. Go do that then.")
                exit(1)

        RunSubprocess(["git", "checkout", tag_effective])

    if args.resume == 0:
        print("Pulling the necessary images...")
        RunSubprocess(["make", "-C", "tools/docker/golang", "pull-all"])

    if platform == "mac":
        if args.resume <= 10:
            print("Deploying binaries...")
            RunSubprocess(["make", "-C", "mos", "deploy-mos-binary", "TAG=%s" % tag_effective])
        repo = git.Repo(".")
        head_commit = repo.head.commit
        formula = ("mos" if args.release_tag != "" else "mos-latest")
        v = json.load(open(os.path.expanduser("~/tmp/mos_gopath/src/cesanta.com/mos/version/version.json"), "r"))
        hb_cmd = [
            "tools/update_hb.py",
            "--hb-repo=git@github.com:cesanta/homebrew-mos.git",
            "--formula=%s" % formula,
            "--blob-url=https://github.com/cesanta/mos-tool/archive/%s.tar.gz" % head_commit,
            "--version=%s" % v["build_version"],
            "--commit", "--push",
        ]
        if args.resume <= 20:
            print("Updating Homebrew...")
            RunSubprocess(hb_cmd)
        if args.resume <= 30:
            print("Building a bottle...")
            # We've just updated the formula.
            RunSubprocess(["brew", "update"])
            RunSubprocess(["brew", "uninstall", "-f", "mos", "mos-latest"])
            RunSubprocess(["brew", "install", "--build-bottle", formula])
            out = RunSubprocess(["brew", "bottle", formula]).decode("utf-8")
            ll = [l for l in out.splitlines() if not l.startswith("==")]
            bottle_fname = ll[0]
            hb_cmd.extend([
                "--bottle=%s" % bottle_fname,
                "--bottle-upload-dest=core@mongoose-os.com:/data/downloads/homebrew/bottles-%s/" % formula
            ])
            RunSubprocess(hb_cmd)

    if args.resume <= 40:
        RunSubprocess(["make", "-C", "mos", "deploy-fwbuild", "TAG=%s" % tag_effective])

    if args.resume <= 50:
        for i, distr in enumerate(UBUNTU_VERSIONS):
            RunSubprocess(["/bin/bash", BUILD_DEB_PATH, deb_package, distr, args.release_tag])

    if args.resume <= 60:
        for i, distr in enumerate(UBUNTU_VERSIONS):
            RunSubprocess(["/bin/bash", UPLOAD_DEB_PATH, deb_package, distr], communicator=UploaderComm)

    if platform != "mac":
        print("""
    ============ WARNING ============
    You're not running on mac, so I couldn't deploy mos binary. You need to do that from mac:
    $ make -C mos deploy-mos-binary TAG=%s
    =================================""" % tag_effective)
