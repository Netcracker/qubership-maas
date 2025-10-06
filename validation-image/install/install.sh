#!/bin/sh
set -e
set -x

apk add --no-cache zip unzip jq postgresql wget curl bash python3

mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

cp /tmp/validation.sh /validation.sh
cp /tmp/bootstrap.sh /bootstrap.sh
cp -r /tmp/scripts /scripts
chmod +x /validation.sh
chmod +x /bootstrap.sh
chmod -R +x /scripts

GSDK=https://dl.k8s.io/release
curl -L "${GSDK}"/v1.29.0/bin/linux/amd64/kubectl  -o /bin/kubectl
chmod +x /bin/kubectl*

exit 0
