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
import subprocess
import sys

import git  # apt-get install python3-git || pip3 install GitPython

# Not used in this script but is used by other scripts.
# Importing now will catch missing dependencies early.
import github_api

GPG_KEY_PATH = os.path.join(os.environ["HOME"], ".gnupg-cesantabot")
BUILD_DEB_PATH = os.path.join("mos", "ubuntu", "build-deb.sh")
UPLOAD_DEB_PATH = os.path.join("mos", "ubuntu", "upload-deb.sh")
UBUNTU_VERSIONS = ["xenial", "bionic", "cosmic"]

deb_package = "mos-latest"
tag_effective = "latest"

def RunSubprocess(cmd, communicator=None, quiet=False):
    master_fd, slave_fd = pty.openpty()

    print("Running subprocess: %s" % " ".join(cmd))

  #-- run mdb as a child process, and pipe its stdin / stdout / stderr.
    proc = subprocess.Popen(
            cmd,
            stdin=slave_fd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE)

    line = b""

    while not quiet:
        #-- get next byte from mdb's stdout
        byte = proc.stdout.read(1)

        if byte != b"":

            line += byte

            if byte == b"\n":
                sys.stdout.write("[out] " + line.decode("utf-8"))
                line = b""

            if communicator != None:
                communicator(line, master_fd)

        else:
            sys.stdout.write("Child process exited\n")
            break

    proc.wait()
    if proc.returncode != 0:
        print("returncode: %d" % proc.returncode)
        print("stderr: %s" % proc.stderr.read())
        raise Exception("non-zero return code")


def UploaderComm(line, tty):
    if line == b"Enter passphrase: ":
        os.write(tty, bytes(passphrase + "\n", "utf-8"))
        sys.stdout.write("\n[sent passphrase]\n")
    elif line == b"Would you like to use the current signature? [Yn]":
        os.write(tty, bytes("y\n", "utf-8"))
        sys.stdout.write("\n[sent y]\n")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()

    parser.add_argument(
            '--release-tag',
            required = False,
            default = "",
            help = 'Release tag, like 1.12')

    myargs = parser.parse_args()

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

    # Request the user for the passphrase
    passphrase = getpass.getpass("Passphrase for the key in %s: " % GPG_KEY_PATH)
    print("Checking passphrase...")
    RunSubprocess([
        "docker", "run", "-it", "--rm",
        "-v", "%s:/root/.gnupg" % GPG_KEY_PATH,
        "docker.io/mgos/ubuntu-golang:xenial",
        "gpg", "--sign", "--no-use-agent", "-o", "/dev/null", "/dev/null"],
        communicator=UploaderComm)
    print("Ok, passphrase is correct")

    if myargs.release_tag != "":
        deb_package = "mos"
        tag_effective = myargs.release_tag

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

    RunSubprocess(["make", "-C", "tools/docker/golang", "pull-all"])

    if platform == "mac":
        print("Deploying Mac binary...")
        RunSubprocess(["make", "-C", "mos", "deploy-mos-binary", "TAG=%s" % tag_effective])
        print("Updating Homebrew...")
        repo = git.Repo(".")
        head_commit = repo.head.commit
        v = json.load(open(os.path.expanduser("~/tmp/mos_gopath/src/cesanta.com/mos/version/version.json"), "r"))
        RunSubprocess([
            "tools/update_hb.py",
            "--hb-repo=git@github.com:cesanta/homebrew-mos.git",
            "--formula=%s" % ("mos" if myargs.release_tag != "" else "mos-latest"),
            "--blob-url=https://github.com/cesanta/mos-tool/archive/%s.tar.gz" % head_commit,
            "--version=%s" % v["build_version"],
            "--push",
        ])

    RunSubprocess(["make", "-C", "mos", "deploy-fwbuild", "TAG=%s" % tag_effective])

    for i, distr in enumerate(UBUNTU_VERSIONS):
        RunSubprocess(
                ["/bin/bash", BUILD_DEB_PATH, deb_package, distr, myargs.release_tag]
        )

    for i, distr in enumerate(UBUNTU_VERSIONS):
        RunSubprocess(
                ["/bin/bash", UPLOAD_DEB_PATH, deb_package, distr],
                communicator=UploaderComm
        )

    if platform != "mac":
        print("""
    ============ WARNING ============
    You're not running on mac, so I couldn't deploy mos binary. You need to do that from mac:
    $ make -C mos deploy-mos-binary TAG=%s
    =================================""" % tag_effective)
