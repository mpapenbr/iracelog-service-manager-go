#!/bin/bash
echo "devinit.sh called"

make install

if [ -f setuplinks.sh ]; then
    . ./setuplinks.sh
fi

# keep this here in comments as a source if we need to install it on demand

# go install github.com/goreleaser/goreleaser/v2@latest
# go install github.com/caarlos0/svu@latest
