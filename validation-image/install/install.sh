#!/bin/sh
set -e
set -x

# Alpine edge: python3 >= 3.14.7, postgresql >= 18.5, nghttp2 >= 1.70.0
echo "https://dl-cdn.alpinelinux.org/alpine/edge/main" > /etc/apk/repositories
echo "https://dl-cdn.alpinelinux.org/alpine/edge/community" >> /etc/apk/repositories
apk upgrade --no-cache
apk add --no-cache \
  "postgresql>=18.5-r0" \
  "python3>=3.14.7-r0" \
  "nghttp2-libs>=1.70.0-r0" \
  curl bash

mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

cp /tmp/validation.sh /validation.sh
cp /tmp/bootstrap.sh /bootstrap.sh
cp -r /tmp/scripts /scripts
chmod +x /validation.sh
chmod +x /bootstrap.sh
chmod -R +x /scripts

GSDK=https://dl.k8s.io/release
curl -L "${GSDK}"/v1.36.2/bin/linux/amd64/kubectl  -o /bin/kubectl
chmod +x /bin/kubectl*

exit 0
