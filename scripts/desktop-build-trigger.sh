#!/bin/bash
# Trigger a one-shot desktop image build on the homelab cluster without
# applying anything from this repo: the TaskRun template embedded in the
# desktop-build-taskrun-template ConfigMap (deployed by
# clusters/homelab/apps/cicd-builds/desktop-build/cronjob-desktop-build.yaml)
# is extracted and fed to `kubectl create -f -` — exactly what the weekly
# build-desktop-weekly CronJob does, just from the workstation. The template
# uses generateName, so the script can be run repeatedly.
#
# The cluster's ConfigMap is the source of truth: local, unapplied edits to
# the template files do not affect builds started this way. The template pins
# GIT_REVISION to main (override with -r) and pushes
# ghcr.io/frzifus/desktop:latest (latest-arm64 for arm64).
#
# Note the ConfigMap template differs from taskrun-desktop-build.yaml: no
# STORAGE_DRIVER: vfs (uses the Task default overlay), no hostUsers, no
# resource limits. Scheduled builds run this exact variant every week.
#
# The arm64 template ConfigMap only exists while cronjob-desktop-build-arm64.yaml
# is enabled in clusters/homelab/apps/cicd-builds/desktop-build/kustomization.yaml
# (currently commented out).
#
# Usage:
#   desktop-build-trigger.sh               # amd64 build of main
#   desktop-build-trigger.sh arm64         # arm64 build of main
#   desktop-build-trigger.sh -r feature-x  # amd64 build of feature-x

set -euo pipefail

namespace=cicd-builds
arch=amd64
revision=main

usage() {
	echo "usage: $(basename "$0") [amd64|arm64] [-r <git-revision>]" >&2
	exit "$1"
}

while [ $# -gt 0 ]; do
	case "$1" in
	amd64 | arm64) arch=$1; shift ;;
	-r | --revision)
		[ $# -ge 2 ] || usage 1
		revision=$2
		shift 2
		;;
	-h | --help) usage 0 ;;
	*) usage 1 ;;
	esac
done

case $arch in
amd64) configmap=desktop-build-taskrun-template ;;
arm64) configmap=desktop-build-arm64-taskrun-template ;;
esac

template=$(kubectl get cm "$configmap" -n "$namespace" -o jsonpath='{.data.taskrun\.yaml}' 2>/dev/null) || {
	echo "error: ConfigMap $namespace/$configmap not found" >&2
	if [[ $arch == arm64 ]]; then
		echo "the arm64 template is only deployed while cronjob-desktop-build-arm64.yaml" >&2
		echo "is enabled in clusters/homelab/apps/cicd-builds/desktop-build/kustomization.yaml" >&2
	fi
	exit 1
}

if [[ $revision != main ]]; then
	template=$(awk -v rev="$revision" '
		/- name: GIT_REVISION/ { print; if ((getline) <= 0) exit; sub(/value:.*/, "value: " rev); print; next }
		{ print }
	' <<<"$template")
fi

created=$(printf '%s\n' "$template" | kubectl create -o name -f -)
run=${created##*/}

echo "started $run  ($arch, revision: $revision)"
echo
echo "follow:"
echo "  kubectl get taskrun -n $namespace $run -w"
echo "  kubectl logs -f -n $namespace $run-pod -c step-build-and-push"