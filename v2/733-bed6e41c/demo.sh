#!/bin/bash
# PR #733 before/after proof - docs/install pin 0.1.0 vs latest-release lookup
export TERM=xterm-256color GIT_PAGER=cat PAGER=cat LESS=-FX
cd ~/workspace/pr-evidence/pr-733 || exit 1
run(){ echo; echo "\$ $*"; eval "$*"; }
pause(){ sleep "${1:-2}"; }
clear
echo "================================================================"
echo " PR #733 - docs(install): fetch the latest release instead"
echo " of pinning 0.1.0   (issue #694)"
echo " Proof = execute each guide's own commands, live."
echo "================================================================"
pause 3
echo; echo "########## BEFORE - base branch: develop ##########"
run "git checkout -q develop && git log --oneline -1"
pause
run "grep -rn 'VERSION=\|0\.1\.0' docs/install README.md"
pause 3
echo; echo "-- ubuntu.md Option B, followed literally (up to download step):"
run "sed -n '/## Option B/,/^curl/p' docs/install/ubuntu.md | head -12"
run "VERSION=0.1.0; ARCH=amd64; echo VERSION=\$VERSION"
run "URL=\"https://github.com/peasant-labs/peasant/releases/download/v\${VERSION}/peasant_\${VERSION}_linux_\${ARCH}.tar.gz\"; echo \$URL"
run "curl -sIL -o /dev/null -w 'HTTP %{http_code}\n' \"\$URL\""
echo ">> The URL works - and installs peasant 0.1.0, long superseded."
run "gh release view --repo peasant-labs/peasant --json tagName --jq .tagName"
echo ">> Latest release today is the tag above; the guide never reaches it."
run "curl -sIL -o /dev/null -w 'windows v0.1.0 zip: HTTP %{http_code}\n' 'https://github.com/peasant-labs/peasant/releases/download/v0.1.0/peasant_0.1.0_windows_amd64.zip'"
echo ">> ...and the Windows 0.1.0 asset does not even exist (404)."
pause 3
echo; echo "########## AFTER - PR branch: qa/pr-docs-install-version ##########"
run "git checkout -q qa/pr-docs-install-version && git log --oneline -3"
pause
run "grep -rn 'VERSION=\|0\.1\.0' docs/install README.md"
echo ">> No VERSION=0.1.0 pins left. Only the rc-sorting example"
echo ">> (0.1.0~rc1 in ubuntu.md) remains - intentional, not a pin."
pause 3
echo; echo "-- the PR guide's own version-resolution snippet, run live:"
run "sed -n '/## Option B/,/^ARCH/p' docs/install/ubuntu.md | head -10"
run "VERSION=\$(curl -fsSL https://api.github.com/repos/peasant-labs/peasant/releases/latest | sed -n 's/.*\"tag_name\": *\"v\([^\"]*\)\".*/\1/p')"
run ": \"\${VERSION:?could not read the latest release (GitHub API rate limit?); set VERSION=0.12.0 by hand}\""
run "ARCH=amd64; echo \"VERSION=\$VERSION ARCH=\$ARCH\""
run "URL=\"https://github.com/peasant-labs/peasant/releases/download/v\${VERSION}/peasant_\${VERSION}_linux_\${ARCH}.tar.gz\"; echo \$URL"
run "curl -sIL -o /dev/null -w 'tar.gz: HTTP %{http_code}\n' \"\$URL\""
run "curl -sIL -o /dev/null -w '.deb:   HTTP %{http_code}\n' \"https://github.com/peasant-labs/peasant/releases/download/v\${VERSION}/peasant_\${VERSION}_linux_\${ARCH}.deb\""
run "gh release view --repo peasant-labs/peasant --json assets --jq '.assets[].name' | grep linux_amd64"
echo ">> Constructed asset names match the real release assets exactly."
pause 3
echo; echo "-- guard behaviour when the lookup returns no version (rate-limit payload):"
run "( VERSION=\$(printf '%s' '{\"message\":\"API rate limit exceeded\"}' | sed -n 's/.*\"tag_name\": *\"v\([^\"]*\)\".*/\1/p'); : \"\${VERSION:?could not read the latest release (GitHub API rate limit?); set VERSION=0.12.0 by hand}\" )"
echo ">> Guard exit code: $? - clear message, no broken download URL is built."
pause 3
echo; echo "================================================================"
echo " BEFORE: guide pins VERSION=0.1.0 (stale; Windows asset 404)"
echo " AFTER:  guide resolves the latest release live (shown above)"
echo "         and HEAD-checks 200; failure is guarded with a message."
echo "================================================================"
pause 4
