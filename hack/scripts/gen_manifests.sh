#!/usr/bin/env bash

echo "==> Generate manifests"

go run sigs.k8s.io/controller-tools/cmd/controller-gen \
		rbac:roleName=controller-manager-role \
		crd:ignoreUnexportedFields=true \
		webhook \
		paths="./..." \
		output:crd:artifacts:config=config/crd/bases

