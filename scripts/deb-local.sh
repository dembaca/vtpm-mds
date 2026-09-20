#!/usr/bin/env bash
# Build the local host and guest .debs with a Version that encodes the git
# commit, so Hogan (and other lab hosts) can tell exactly which tree is
# installed.
# Does not rewrite debian/changelog in the working tree.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ ! -d debian || ! -f debian/changelog ]]; then
  echo "error: debian/changelog not found" >&2
  exit 1
fi

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "error: not a git checkout" >&2
  exit 1
fi

base="$(dpkg-parsechangelog -S Version)"
if [[ "$base" == *+git* ]]; then
  echo "error: debian/changelog is already git-stamped ($base); restore the released changelog first" >&2
  exit 1
fi

sha="$(git rev-parse --short=12 HEAD)"
commit_date="$(git show -s --format=%cd --date=format:%Y%m%d HEAD)"
branch="$(git rev-parse --abbrev-ref HEAD)"
dirty=""
if [[ -n "$(git status --porcelain)" ]]; then
  dirty=".dirty"
fi
local_ver="${base}+git${commit_date}.${sha}${dirty}"
maintainer="$(dpkg-parsechangelog -S Maintainer)"

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

cat >"$tmp" <<EOF
vtpm-mds (${local_ver}) UNRELEASED; urgency=medium

  * Local lab build from git ${sha} (${branch}${dirty:+, dirty working tree}).
    Not for publication. Identify with: dpkg -l vtpm-mds; vtpm-mds -version

 -- ${maintainer}  $(date -R)

EOF
cat debian/changelog >>"$tmp"

# Stage the stamped changelog only inside the make deb copy.
# make deb tars the working tree, so swap changelog around that call.
orig="debian/changelog"
bak="debian/changelog.deb-local.bak"
cp -a "$orig" "$bak"
cp "$tmp" "$orig"
restore() {
  if [[ -f "$bak" ]]; then
    mv -f "$bak" "$orig"
  fi
}
trap 'restore; rm -f "$tmp"' EXIT

echo "Building local packages at version ${local_ver}"
make deb
restore
trap 'rm -f "$tmp"' EXIT

host_deb="dist/vtpm-mds_${local_ver}_amd64.deb"
guest_deb="dist/devid-enroll_${local_ver}_amd64.deb"

echo
echo "Local lab packages:"
ls -lh "$host_deb" "$guest_deb"
echo "Install the host daemon on the lab host with:"
echo "  dpkg -i --force-confold ${host_deb}"
echo "  systemctl start vtpm-mds"
echo "  vtpm-mds -version"
echo "Install the guest client in a VM image with:"
echo "  dpkg -i ${guest_deb}"
echo "  devid-enroll -version"
