#!/usr/bin/env bash
# Render the datacow Homebrew formula to stdout.
#
# Usage: scripts/gen-formula.sh <url> <sha256>
#
# Homebrew derives the version from the tarball name in <url>
# (datacow-1.2.3.tar.gz -> 1.2.3) and the formula injects it at build time
# via -X main.version. Shared by the release workflow and the CI formula check
# so both exercise the exact same generator.
#
# Note: #{...} are Ruby interpolations evaluated by Homebrew; the unquoted
# heredoc only expands $-prefixed shell vars, so they pass through verbatim.
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <url> <sha256>" >&2
  exit 2
fi

url="$1"
sha="$2"

cat <<EOF
class Datacow < Formula
  desc "Zero-config terminal database explorer, like k9s or lazygit but for databases"
  homepage "https://github.com/polesen/datacow"
  url "${url}"
  sha256 "${sha}"
  license "MIT"
  head "https://github.com/polesen/datacow.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=#{version}"), "./cmd"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/datacow --version")
  end
end
EOF
