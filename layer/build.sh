#!/usr/bin/env bash

set -e

GEMS=("github-pages")
IMAGE="amazonlinux:2018.03"
OPENSSH_VER="8.0p1"
RSYNC_VER="3.1.3"
RUBY_VER="2.6.3"  # If upgrading beyond 2.6.x, make sure to replace any occurences of 2.6.0.

if [ "$1" = build ]; then
  # This bit runs inside a Docker container.
  # Install required packages for building stuff.
  yum -y update
  yum -y install bzip2 findutils gcc-c++ openssl-devel patch perl readline-devel zlib-devel

  # Clean up any artifacts from previous builds.
  for f in /opt/*; do
    [[ "$f" = "$0" ]] || rm -rf "$f"
  done

  # Build and install rsync.
  curl "https://download.samba.org/pub/rsync/rsync-$RSYNC_VER".tar.gz | tar zxf -
  cd "rsync-$RSYNC_VER"
  ./configure --prefix=/opt
  make
  make install
  cd ..

  # Build and install openssh.
  curl "https://cloudflare.cdn.openbsd.org/pub/OpenBSD/OpenSSH/portable/openssh-$OPENSSH_VER.tar.gz" | tar zxf -
  cd "openssh-$OPENSSH_VER"
  ./configure --prefix=/opt
  make
  make install
  cd ..

  # Build and install Ruby.
  curl -L "https://github.com/rbenv/ruby-build/archive/v20190615.tar.gz" | tar zxf -
  RUBY_CONFIGURE_OPTS="--enable-shared" ./ruby-build-20190615/bin/ruby-build --verbose "$RUBY_VER" /opt

  # Install required gems.
  /opt/bin/gem install "${GEMS[@]}"

  # Delete build artifacts we don't need to minimize layer size.
  rm -rf /opt/{include,share}
  rm -rf /opt/lib/ruby/gems/**/{cache,doc}

  # Fix a symlink that will cause AWS deployment to fail.
  cd /opt/lib/ruby/gems/2.6.0/gems/ffi-1.11.1/ext/ffi_c
  cp --remove-destination libffi/src/x86/ffitarget.h libffi-x86_64-linux/include/
else
  # This bit runs in your own shell.
  cd $(dirname "$0")

  # Avoid rebuilding when possible.
  if [ -d bin ]; then
    [[ "$1" = "no" ]] && exit
    read -e -p "Layer '$(basename $(pwd))' appears to be already built. Rebuild? [y/N]> " confirm
    [[ "$confirm" != [Yy]* ]] && exit
  fi

  script=$(basename "$0")

  # Run this same script in Docker.
  docker run -t --rm --mount "type=bind,source=$(pwd),destination=/opt" "$IMAGE" "/opt/$script" build
fi
