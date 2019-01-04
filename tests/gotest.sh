#!/bin/bash

GOPACKAGES="$(go list ./... | grep -v vendor)"
exec go test ${GOPACKAGES} ${1}
