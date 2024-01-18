#!/bin/bash

set -e

this_dir="$(cd $(dirname $0) && pwd)"

pushd "$this_dir"

rm -rf out
certstrap init --common-name "ca" --passphrase "" --expires "10 years"
certstrap request-cert --common-name "server" --domain "localhost" --ip "127.0.0.1" --passphrase ""
certstrap sign server --CA "ca" --expires "10 years"
certstrap request-cert --common-name "server-bad" --passphrase ""
certstrap sign server-bad --CA "ca" --expires "10 years"

mv -f out/* ./
rm -rf out

popd
