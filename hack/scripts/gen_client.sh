#!/usr/bin/env bash

if [ $# -ne 1 ]; then
    echo "project root is expected"
fi

PROJECT_ROOT="$1"
PROJECT_PKG="github.com/opendatahub-io/opendatahub-operator/v2"
TMP_DIR=$( mktemp -d -t odh-operator-client-gen-XXXXXXXX )

mkdir -p "${TMP_DIR}/client"
mkdir -p "${PROJECT_ROOT}/pkg/client/dscinitialization"

#echo "tmp dir: $TMP_DIR"

echo "==> Generate ApplyConfiguration"
go run k8s.io/code-generator/cmd/applyconfiguration-gen \
  --go-header-file="${PROJECT_ROOT}/hack/boilerplate.go.txt" \
  --output-base="${TMP_DIR}/client" \
  --output-package=${PROJECT_PKG}/pkg/client/applyconfiguration \
  --input-dirs=${PROJECT_PKG}/apis/dscinitialization/v1,${PROJECT_PKG}/apis/datasciencecluster/v1,${PROJECT_PKG}/apis/features/v1

echo "==> Generate client"
go run k8s.io/code-generator/cmd/client-gen \
  --go-header-file="${PROJECT_ROOT}/hack/boilerplate.go.txt" \
  --output-base="${TMP_DIR}/client" \
  --input-base=${PROJECT_PKG}/apis \
  --input=dscinitialization/v1,datasciencecluster/v1,features/v1 \
  --fake-clientset=false \
  --clientset-name "versioned" \
  --apply-configuration-package=${PROJECT_PKG}/pkg/client/applyconfiguration \
  --output-package=${PROJECT_PKG}/pkg/client/clientset

echo "==> Generate lister"
go run k8s.io/code-generator/cmd/lister-gen \
  --go-header-file="${PROJECT_ROOT}/hack/boilerplate.go.txt" \
  --output-base="${TMP_DIR}/client" \
  --output-package=${PROJECT_PKG}/pkg/client/listers \
  --input-dirs=${PROJECT_PKG}/apis/dscinitialization/v1,${PROJECT_PKG}/apis/datasciencecluster/v1,${PROJECT_PKG}/apis/features/v1

echo "==> Generate informer"
go run k8s.io/code-generator/cmd/informer-gen \
  --go-header-file="${PROJECT_ROOT}/hack/boilerplate.go.txt" \
  --output-base="${TMP_DIR}/client" \
  --versioned-clientset-package=${PROJECT_PKG}/pkg/client/clientset/versioned \
  --listers-package=${PROJECT_PKG}/pkg/client/listers \
  --output-package=${PROJECT_PKG}/pkg/client/informers \
  --input-dirs=${PROJECT_PKG}/apis/dscinitialization/v1,${PROJECT_PKG}/apis/datasciencecluster/v1,${PROJECT_PKG}/apis/features/v1

# This should not be needed but for some reasons, the applyconfiguration-gen tool
# sets a wrong APIVersion.
#
# See: https://github.com/kubernetes/code-generator/issues/150
sed -i \
  's/WithAPIVersion(\"dscinitialization\/v1\")/WithAPIVersion(\"dscinitialization.opendatahub.io\/v1\")/g' \
  "${TMP_DIR}"/client/${PROJECT_PKG}/pkg/client/applyconfiguration/dscinitialization/v1/dscinitialization.go
sed -i \
  's/WithAPIVersion(\"datasciencecluster\/v1\")/WithAPIVersion(\"datasciencecluster.opendatahub.io\/v1\")/g' \
  "${TMP_DIR}"/client/${PROJECT_PKG}/pkg/client/applyconfiguration/dscinitialization/v1/dscinitialization.go
sed -i \
  's/WithAPIVersion(\"features\/v1\")/WithAPIVersion(\"features.opendatahub.io\/v1\")/g' \
  "${TMP_DIR}"/client/${PROJECT_PKG}/pkg/client/applyconfiguration/features/v1/featuretracker.go

rm -rf \
  "${PROJECT_ROOT}"/pkg/client/*
cp -r \
  "${TMP_DIR}"/client/${PROJECT_PKG}/pkg/client/* \
  "${PROJECT_ROOT}"/pkg/client

