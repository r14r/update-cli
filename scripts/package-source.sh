#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
VERSION="$(tr -d '[:space:]' < "$ROOT_DIR/VERSION")"
NAME="update-cli-v${VERSION}"
OUTPUT="${1:-$ROOT_DIR/../${NAME}.zip}"
STAGE="$(mktemp -d "${TMPDIR:-/tmp}/update-cli-package.XXXXXX")"
trap 'rm -rf "$STAGE"' EXIT

case "$VERSION" in
  [0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "ERROR invalid VERSION: $VERSION" >&2; exit 1 ;;
esac

mkdir -p "$STAGE/$NAME"
rsync -a \
  --exclude='/.git/' \
  --exclude='/.update-cli/' \
  --exclude='/dist/' \
  --exclude='/.demo-install/' \
  --exclude='/.demo-quickstart/' \
  --exclude='/docs/videos/' \
  --exclude='/update-cli' \
  --exclude='/update-cli-*' \
  --exclude='/*.zip' \
  --exclude='/.DS_Store' \
  "$ROOT_DIR/" "$STAGE/$NAME/"

required=(
  VERSION README.md RELEASE_NOTES.md go.mod main.go update-cli.yaml
  defaults/config.json defaults/templates.json
  docs/tapes/install.tape docs/tapes/quickstart.tape scripts/render-tapes.sh scripts/validate-version.sh
  lib/backup/backup.go lib/config/config.go lib/updater/updater.go lib/rsync/rsync.go
)
for rel in "${required[@]}"; do
  [[ -f "$STAGE/$NAME/$rel" ]] || { echo "ERROR required source file missing: $rel" >&2; exit 1; }
done

[[ ! -e "$STAGE/$NAME/.update-cli" ]] || { echo 'ERROR .update-cli must not be packaged' >&2; exit 1; }
[[ ! -e "$STAGE/$NAME/dist" ]] || { echo 'ERROR dist must not be packaged' >&2; exit 1; }
[[ ! -e "$STAGE/$NAME/RELEASE_VERSION" ]] || { echo 'ERROR RELEASE_VERSION must not be packaged; VERSION is the single source' >&2; exit 1; }

rm -f "$OUTPUT"
(
  cd "$STAGE"
  zip -qr "$OUTPUT" "$NAME"
)

archive_version="$(unzip -p "$OUTPUT" "$NAME/VERSION" | tr -d '[:space:]')"
[[ "$archive_version" == "$VERSION" ]] || { echo "ERROR archive VERSION=$archive_version expected=$VERSION" >&2; exit 1; }
LISTING="$STAGE/archive-list.txt"
unzip -l "$OUTPUT" > "$LISTING"
grep -q " $NAME/lib/backup/backup.go$" "$LISTING" || { echo 'ERROR archive missing lib/backup/backup.go' >&2; exit 1; }
if grep -Eq " $NAME/(\.update-cli|dist)(/|$)" "$LISTING"; then
  echo 'ERROR archive contains runtime/build state' >&2
  exit 1
fi
if grep -Eq " $NAME/RELEASE_VERSION$" "$LISTING"; then
  echo 'ERROR archive contains obsolete RELEASE_VERSION' >&2
  exit 1
fi
printf '%s\n' "$OUTPUT"
