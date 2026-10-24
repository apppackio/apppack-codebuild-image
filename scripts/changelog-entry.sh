#!/bin/sh
# Record a dependency bump in CHANGELOG.md under ## [Unreleased].
#
# Called by Renovate's postUpgradeTasks (see renovate.json) so that user-facing
# tool bumps land in the changelog with the bump itself, instead of being
# reconstructed from git log at release time.
#
# Only commands matching RENOVATE_ALLOWED_COMMANDS in
# .github/workflows/renovate.yml are allowed to run, so the argument shape here
# and the regex there have to stay in step.
#
# Renovate re-runs this on every rebase of the branch, so re-running with a
# different version replaces the existing entry instead of stacking a second
# one.
#
# usage: scripts/changelog-entry.sh pack <version>

set -eu

CHANGELOG=CHANGELOG.md

usage() {
	echo "usage: $0 pack <version>" >&2
	exit 64
}

[ $# -eq 2 ] || usage
[ "$1" = pack ] || usage

version=$2
# Guard the one value that reaches the file, since it arrives from a Renovate
# template rather than from this repo.
case $version in
*[!0-9.]* | '' | *..*) usage ;;
esac

[ -f "$CHANGELOG" ] || {
	echo "$0: $CHANGELOG not found (run from the repo root)" >&2
	exit 1
}

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

# Read the whole file first, then edit by line number: the insertion point
# depends on what the Unreleased section already contains, which a streaming
# pass cannot know until it has gone too far.
awk -v entry="* Updated the \`pack\` CLI to $version" '
{ line[NR] = $0 }

END {
    for (i = 1; i <= NR; i++) {
        if (line[i] ~ /^## \[Unreleased\]/) { start = i; break }
    }
    if (!start) { print "no ## [Unreleased] heading" > "/dev/stderr"; exit 1 }

    # The section runs to the next release heading, or to EOF.
    end = NR + 1
    for (i = start + 1; i <= NR; i++) {
        if (line[i] ~ /^## \[/) { end = i; break }
    }

    # An entry for this tool already there: swap the version in place so the
    # bullet keeps its position.
    for (i = start + 1; i < end; i++) {
        if (line[i] ~ /^\* Updated the `pack` CLI to /) { line[i] = entry; at = -1; break }
    }

    # Otherwise insert at the top of ### Changed, adding the section if the
    # Unreleased block does not have one yet.
    if (!at) {
        for (i = start + 1; i < end; i++) {
            if (line[i] ~ /^### Changed/) { at = i; break }
        }
        if (at) {
            # Step over the blank line under the heading so the new bullet
            # joins the list rather than splitting it.
            j = at + 1
            while (j < end && line[j] == "") j++
            after[j - 1] = entry
        } else {
            after[start] = "\n### Changed\n\n" entry
        }
    }

    for (i = 1; i <= NR; i++) {
        print line[i]
        if (i in after) print after[i]
    }
}
' "$CHANGELOG" >"$tmp"

# awk wrote to a temp file so a failure midway cannot truncate the changelog.
if ! grep -q '^\* Updated the `pack` CLI to ' "$tmp"; then
	echo "$0: failed to insert entry into $CHANGELOG" >&2
	exit 1
fi

cat "$tmp" >"$CHANGELOG"
echo "$0: recorded pack $version in $CHANGELOG"
