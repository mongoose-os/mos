# How to deploy latest firmware and mos:

  * Log in to the Mac machine (you may do that from linux, but standalone
    binaries won't be deployed then). You can use a mac in the office, see
    the section in the end
  * From the root of the dev repo, invoke:

```bash
$ tools/deploy_mos.py
```
  * Done!

# How to cut a release

  * Make sure all repos are published properly, you can check the logs:

```bash
$ sudo journalctl -n 1 -u publish_repos.service
-- Logs begin at Thu 2017-08-03 02:30:43 UTC, end at Wed 2017-08-16 12:48:04 UTC. --
Aug 16 12:33:38 ci.c.cesanta-2015.internal systemd[1]: Started Publishes public repos.
```

  * Stop publishing service on the CI machine:

```bash
$ sudo systemctl stop publish_repos.timer
```

  * From some machine with fast Internet access, create tags and github
    releases for the new version (from the dev repo root)

```bash
$ tools/make_release_tags.py --release-tag X.Y.Z
```

  * Previous command has created a tag X.XX on all our repos. Now you need to
    provide a release notes at least on mongoose-os and mos-tool repos; for
    that, use `tools/extract_changelog.py --no-pull --from A.B.C --to X.Y.Z`,
    where `A.B.C` is the previous release.
    The output is rough, so you need to "file it down" a bit, and then
    publish as release notes on mongoose-os and mos-tool repos (you can do that
    in parallel with the `tools/deploy_mos.py` command below, which takes a
    long time)

  * Make sure you're on mac (You can use a mac in the office, see the section
    in the end)

  * From the root of the dev repo, invoke:

```bash
$ tools/deploy_mos.py --release-tag X.XX
```

  * Restore publishing service on the CI machine:

```bash
$ sudo systemctl start publish_repos.timer
```

  * Update arch linux PKGBUILD (`mos/archlinux_pkgbuild/mos-release/PKGBUILD`):
    bump `MOS_TAG` to a new version and make a PR.

  * You're done! And if the process went without any bumps, keep in mind that
    today must be a lucky day for you!

# Details of building Ubuntu packages

Building and uploading of debs is covered by the `tools/deploy_mos.py` script
explained in the section above, but for reference, here are details.

We have a PPA on Launchpad called [mongoose-os](https://launchpad.net/~mongoose-os/+archive/ubuntu/mos).

PPAs are cool in that they free us from the need of hosting a Debian repo, but they are restrictive too: binary packages must be built from a source deb on their build machines.
This is fine because all of `mos` is open source anyway, but it restricts things we can do, like for example this is the reason why mos must be buildable with Go 1.6 (version bundled with 16.04 Xenial, which is the current LTS at the time of writing).

Long story short, we managed to get it going, and here's how to do it, followed by the nitty-gritty if you are curious and/or want to understand how things work or why they don't.

## How to do it

 * Get a hold of the `Cesanta Bot`'s GPG keys (`/data/.gnupg-cesantabot @ secure`), put it somewhere on your machine. Let's say, `~/.gnupg-cesantabot`.
 * Get a clone of `cesanta/mongoose-os`. `NB: `cesanta/dev` will not do`
   * No, really, it won't. May look like it should, but it won't. Use the public repo.
   * Even if it does you should not do it, because all of it is archived in the source .deb and we don't want that.
 * Set the `PACKAGE` and `DISTR` variables for use in the commands below:
   * `PACKAGE=mos` or `PACKAGE=mos-latest`
   * `DISTR=xenial` or `DISTR=bionic`
 * Build the source and binary packages (we build `mos-latest` for `xenial` below)
```
[rojer@nbmbp ~/cesanta/mongoose-os master]$ bash mos/ubuntu/build-deb.sh $PACKAGE $DISTR
...
...
   dh_builddeb -u-Zxz -O--buildsystem=golang
dpkg-deb: building package 'mos-latest' in '../mos-latest_201707270849+a2f2ca8~xenial0_amd64.deb'.
 dpkg-genchanges -b >../mos-latest_201707270849+a2f2ca8~xenial0_amd64.changes
dpkg-genchanges: binary-only upload (no source code included)
 dpkg-source --after-build mos-latest
dpkg-buildpackage: binary-only upload (no source included)
Now running lintian...
warning: the authors of lintian do not recommend running it with root privileges!
E: mos-latest: embedded-library usr/bin/mos: libyaml
W: mos-latest: latest-debian-changelog-entry-changed-to-native
W: mos-latest: binary-without-manpage usr/bin/mos
Finished running lintian.
```
   * The output directory should look like this:
```
[rojer@nbmbp ~/cesanta/mongoose-os master]$ ls -la /tmp/out-$DISTR
total 15988
drwxr-xr-x  4 root root     4096 Jul 27 11:16 .
drwxrwxrwt 64 root root    98304 Jul 27 11:17 ..
drwx------  3 root root     4096 Jul 26 16:23 .cache
drwxr-xr-x 14 root root     4096 Jul 27 11:13 mos-latest
-rw-r--r--  1 root root     7253 Jul 27 11:14 mos-latest_201707270849+a2f2ca8~xenial0_amd64.build
-rw-r--r--  1 root root      788 Jul 27 11:14 mos-latest_201707270849+a2f2ca8~xenial0_amd64.changes
-rw-r--r--  1 root root  3710612 Jul 27 11:14 mos-latest_201707270849+a2f2ca8~xenial0_amd64.deb
-rw-r--r--  1 root root     1339 Jul 27 11:14 mos-latest_201707270849+a2f2ca8~xenial0.dsc
-rw-r--r--  1 root root     1332 Jul 27 11:13 mos-latest_201707270849+a2f2ca8~xenial0_source.build
-rw-r--r--  1 root root     1608 Jul 27 11:14 mos-latest_201707270849+a2f2ca8~xenial0_source.changes
-rw-r--r--  1 root root 12515092 Jul 27 11:13 mos-latest_201707270849+a2f2ca8~xenial0.tar.xz
```
   * (optional) Try installing the binary package: `sudo dpkg -i /tmp/out-$DISTR/*.deb`
 * If everything checks out, it's time to sign and upload to [the PPA](https://launchpad.net/~mongoose-os/+archive/ubuntu/mos).
   * You will be asked for a passphrase for the keyring (twice). You should know who to ask.
```
[rojer@nbmbp ~/cesanta/mongoose-os master]$ DISTR=xenial; bash mos/ubuntu/upload-deb.sh $PACKAGE $DISTR
 signfile mos-latest_201707270849+a2f2ca8~xenial0.dsc Cesanta Bot <cesantabot@cesanta.com>
gpg: WARNING: unsafe ownership on configuration file `/root/.gnupg/gpg.conf'
gpg: WARNING: unsafe ownership on configuration file `/root/.gnupg/gpg.conf'

You need a passphrase to unlock the secret key for
user: "Cesanta Bot <cesantabot@cesanta.com>"
2048-bit RSA key, ID C43EF73A, created 2017-07-16

gpg: gpg-agent is not available in this session

 signfile mos-latest_201707270849+a2f2ca8~xenial0_source.changes Cesanta Bot <cesantabot@cesanta.com>
gpg: WARNING: unsafe ownership on configuration file `/root/.gnupg/gpg.conf'
gpg: WARNING: unsafe ownership on configuration file `/root/.gnupg/gpg.conf'

You need a passphrase to unlock the secret key for
user: "Cesanta Bot <cesantabot@cesanta.com>"
2048-bit RSA key, ID C43EF73A, created 2017-07-16

gpg: gpg-agent is not available in this session

Successfully signed dsc and changes files
$USER not set, will use login information.
Checking signature on .changes
gpg: WARNING: unsafe ownership on configuration file `/root/.gnupg/gpg.conf'
gpg: Signature made Thu Jul 27 10:14:22 2017 UTC using RSA key ID C43EF73A
gpg: Good signature from "Cesanta Bot <cesantabot@cesanta.com>"
Good signature on /work/mos-latest_201707270849+a2f2ca8~xenial0_source.changes.
Checking signature on .dsc
gpg: WARNING: unsafe ownership on configuration file `/root/.gnupg/gpg.conf'
gpg: Signature made Thu Jul 27 10:14:17 2017 UTC using RSA key ID C43EF73A
gpg: Good signature from "Cesanta Bot <cesantabot@cesanta.com>"
Good signature on /work/mos-latest_201707270849+a2f2ca8~xenial0.dsc.
Uploading to ppa (via ftp to ppa.launchpad.net):
  Uploading mos-latest_201707270849+a2f2ca8~xenial0.dsc: done.
  Uploading mos-latest_201707270849+a2f2ca8~xenial0.tar.xz: done.
  Uploading mos-latest_201707270849+a2f2ca8~xenial0_source.changes: done.
Successfully uploaded packages.
```
 * Repeat for `DISTR=bionic`.

Shortly after upload the package should be queued for building [here](https://launchpad.net/~mongoose-os/+archive/ubuntu/mos/+builds?build_text=&build_state=all) and once finished, will appear in the [package list](https://launchpad.net/~mongoose-os/+archive/ubuntu/mos/+packages).

## How it works

  * [git-build-recipe](https://launchpad.net/git-build-recipe) is used to prepare the source package.
    * The recipes are [here](https://github.com/cesanta/mongoose-os/tree/master/mos/ubuntu). They are identical except for the distro name (I wish they weren't, [maybe some day](https://bugs.launchpad.net/git-build-recipe/+bug/1705591)).
    * The recipe specifies:
      * Clone `/src`, which is a volume-mount into the container and must be a clone of the `cesanta/mongoose-os` repo.
      * Merge in a `deb-latest` branch, which is a separate branch with Debian build metadata and a couple symlinks (see [here](https://github.com/cesanta/mongoose-os/tree/deb-latest)). It's branched off really early so there should never be any conflicts.
      * Pull in all the vendored packages using `govendor`. This will be necessary later, when building the binary.
    * `git-build-recipe` then tweaks Debian metadata to set version and create an automatic changelog entry and creates a source deb.
  * Building the binary package
    * As mentioned, this must be doable by a remote builder which knows nothing about Docker and other useful things.
    * Go packages that do not use any external dependencies that are not packaged for the distro can be built magically with [dh-golang](https://pkg-go.alioth.debian.org/packaging.html). Unfortunately, `mos` does have external dependencies, so we perform [an elaborate dance](https://github.com/cesanta/mongoose-os/blob/deb-latest/debian/rules#L11) to prepare `GOPATH` for the build.
      * We basically construct a `$GOPATH/src` with a single `cesanta.com` package (fortunately, a symlink is enough). `cesanta.com` package will have a `vendor` dir with all the dependencies (synced while building the source package).

# If you don't have mac

Here's how you can use mac in the office:

```bash
$ ssh -A -p 1234 mos@dub.cesanta.com
$ tmux
$ d # start docker machine
```

And the dev repo is here:

```bash
$ cd go/src/cesanta.com
```

