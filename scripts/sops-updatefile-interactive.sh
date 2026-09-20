#!/bin/bash
# Interactively re-encrypt every sops secret in the repo against the current
# .sops.yaml creation rules — adds whichever recipients are missing (e.g.
# keyring / dockingstation YubiKeys). Asks y/n per file, q quits early.
# Files whose recipient set already matches .sops.yaml are not touched
# (counted as "up to date"), so re-running the script is cheap.
#
# Each sops updatekeys DECRYPTS the file first (metadata-only diff: the
# ENC[...] values and data key are preserved). Without the static identity
# in ~/.config/sops/age/keys.txt (endgame-rehearsal mode) every decrypt goes
# through a YubiKey (PIN once per session + one tap per file), and files
# without a yubikey stanza fail outright.

set -u

toplevel=$(git rev-parse --show-toplevel 2>/dev/null) || {
	echo "not inside a git repo" >&2
	exit 1
}
cd "$toplevel"

KEYS="$HOME/.config/sops/age/keys.txt"

if ! grep -q '^AGE-SECRET-KEY' "$KEYS" 2>/dev/null; then
	echo "WARNING: no static identity in $KEYS (endgame-rehearsal mode)."
	echo "  - every updatefile decrypts via YubiKey: PIN once per session, one tap per file"
	echo "  - files WITHOUT a yubikey stanza (most of the repo) fail outright"
	echo
	echo "  restore the static identity for a tap-free run:"
	echo "    grep '^AGE-SECRET-KEY' ~/.config/sops/age/flux-keys.backup >> $KEYS"
	echo
	read -rp "Continue yubikey-only anyway? [y/N] " ans || ans=n
	[[ $ans == [yY]* ]] || exit 0
	echo
fi

# recipients the creation rules want
wanted=$(grep -oE 'age1[a-z0-9]+' .sops.yaml | sort -u)

mapfile -t files < <(
	grep -rl --include='*.yaml' --include='*.yml' --exclude-dir=.git \
		'BEGIN AGE ENCRYPTED FILE' . | sed 's|^\./||' | sort
)

total=${#files[@]}
updated=0 skipped=0 uptodate=0 failed=0 left=0
failed_files=()

echo "$total encrypted file(s) in repo"
echo

i=0
for f in "${files[@]}"; do
	i=$((i + 1))

	have=$(grep -oE 'recipient: age1[a-z0-9]+' "$f" | cut -d' ' -f2 | sort -u)
	missing=$(comm -23 <(echo "$wanted") <(echo "$have"))
	[[ -z $missing ]] && { uptodate=$((uptodate + 1)); continue; }

	# label the missing recipients with their .sops.yaml comments
	names=""
	while IFS= read -r r; do
		n=$(grep -oE "$r( *# *[^ ]+)?" .sops.yaml | sed -E 's/.*# *//' | head -1)
		names+="${names:+, }$n"
	done <<< "$missing"

	printf '%3d/%d  %s\n' "$i" "$total" "$f"
	printf '        missing: %s\n' "$names"
	read -rp '        update? [y/n/q] ' ans || ans=q
	case $ans in
	y|Y)
		if sops updatekeys -y "$f"; then
			updated=$((updated + 1))
			echo "        ✓ updated"
		else
			failed=$((failed + 1))
			failed_files+=("$f")
			echo "        FAILED — see sops output above"
		fi
		;;
	q|Q)
		left=$((total - i + 1))
		echo "quitting — $left file(s) left untouched"
		break
		;;
	*)
		skipped=$((skipped + 1))
		;;
	esac
done

echo
echo "updated:    $updated"
echo "up to date: $uptodate   (recipients already match .sops.yaml)"
echo "skipped:    $skipped"
((left > 0)) && echo "left:       $left   (quit early)"
if ((${#failed_files[@]} > 0)); then
	echo "failed:     ${#failed_files[@]}"
	for f in "${failed_files[@]}"; do
		echo "  $f"
	done
	exit 1
fi