#!/usr/bin/env bash

echo "==> Generate resources"
go run sigs.k8s.io/controller-tools/cmd/controller-gen \
		object:headerFile="hack/boilerplate.go.txt"  \
		paths="./..."
