#!/usr/bin/env bash
set -euo pipefail

COMPONENT="${1:?component required (site|proxy)}"
VERSION="${2:?version required}"
TOKEN="${INFRA_DEPLOY_TOKEN:?INFRA_DEPLOY_TOKEN required}"

case "$COMPONENT" in
  site)
    IMAGE="ghcr.io/bhargavms/mogra-site:${VERSION}"
    MANIFEST="manifests/mogra/deployment.yaml"
    ;;
  proxy)
    IMAGE="ghcr.io/bhargavms/mogra-proxy:${VERSION}"
    MANIFEST="manifests/mogra/proxy-deployment.yaml"
    ;;
  *)
    echo "unknown component: ${COMPONENT}" >&2
    exit 1
    ;;
esac

WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

git clone --depth 1 "https://x-access-token:${TOKEN}@github.com/bhargavms/infra.git" "${WORKDIR}/infra"
cd "${WORKDIR}/infra"

yq -i ".spec.template.spec.containers[0].image = \"${IMAGE}\"" "${MANIFEST}"

git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"

git add "${MANIFEST}"
git commit -m "chore(mogra): bump mogra-${COMPONENT} image to ${VERSION}"
git push origin main
