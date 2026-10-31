#!/bin/bash
set -euo pipefail

identity=$1
identifier=$2
bundle=$3
shift 3

# Reuse the local signing identity without requiring login-keychain prompts.
# Key material and the dedicated keychain password never belong in the repo.
signing_dir="${ADM_SIGNING_DIR:-$HOME/.adm/signing}"
signing_keychain="$signing_dir/build.keychain-db"
password_file="$signing_dir/build-keychain-password"
signing_command=(codesign --force)
if [[ "$identity" != '-' && -f "$signing_keychain" && -f "$password_file" ]]; then
    signing_password=$(<"$password_file")
    security unlock-keychain -p "$signing_password" "$signing_keychain"
    unset signing_password
    signing_command+=(--keychain "$signing_keychain")
fi
# Sign nested code explicitly: --deep signing can resolve a nested identity
# through the login keychain instead of using the dedicated keychain.
for library in "$bundle"/Contents/Resources/*.dylib; do
    [[ -f "$library" ]] || continue
    "${signing_command[@]}" "$@" --sign "$identity" --identifier "$identifier.$(basename "$library" .dylib)" "$library"
done
"${signing_command[@]}" "$@" --sign "$identity" --identifier "$identifier" "$bundle"
codesign --verify --deep --strict "$bundle"
