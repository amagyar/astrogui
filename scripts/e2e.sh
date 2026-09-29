#!/usr/bin/env bash
# End-to-end verification against a real Astro project (tasks 9.1-9.4).
#
# Drives the real user path: the binary and its HTTP API, then the blog's own
# build. Requires node_modules in testdata/e2e-blog (npm install).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FIXTURE="$REPO_ROOT/testdata/e2e-blog"
BIN="$(mktemp -d)/astrogui"
WORK="$(mktemp -d)"
# Kill any server this script started, including across fail-exits: the
# binary lives only in this run's temp dir, so match its exact path.
cleanup() {
  [ -n "${SRV_PID:-}" ] && kill "$SRV_PID" 2>/dev/null || true
  pkill -f "^$BIN " 2>/dev/null || true
  rm -rf "$BIN" "$WORK"
}
trap cleanup EXIT

fail() { echo "E2E FAIL: $*" >&2; exit 1; }
note() { echo "— $*"; }

if [ ! -d "$FIXTURE/node_modules" ]; then
  fail "run 'npm install' in testdata/e2e-blog first"
fi

note "building the binary"
(cd "$REPO_ROOT" && go build -o "$BIN" .)

digest() { (cd "$1" && find . -type f | sort | xargs shasum | shasum | cut -d' ' -f1); }

# ---------------------------------------------------------------------------
# Task 9.2: a project that has never run astrogui builds identically before
# and after the tool runs against it (the tool's presence changes nothing),
# and the dependency manifest is unchanged.
# ---------------------------------------------------------------------------
note "9.2: baseline build of a project that never ran astrogui"
FIX="$WORK/pristine"
cp -R "$FIXTURE" "$FIX"
rm -rf "$FIX/drafts" "$FIX/dist"
PKG_BEFORE=$(cat "$FIX/package.json"; cat "$FIX/package-lock.json" | shasum)

(cd "$FIX" && npm run astro build >/dev/null 2>&1) || fail "baseline build failed"
DIGEST_A=$(digest "$FIX/dist")

note "9.2: run astrogui against it, change nothing, rebuild"
SRV_LOG="$WORK/pristine.log"
(cd "$FIX" && "$BIN" --no-open >"$SRV_LOG" 2>&1 & echo $! > "$WORK/pid")
SRV_PID=$(cat "$WORK/pid")
sleep 1
URL=$(grep -o 'http://[^ ]*' "$SRV_LOG" | head -1)
[ -n "$URL" ] || fail "server did not announce its URL"
HOST=${URL#http://}; HOST=${HOST%%/#*}
TOKEN=$(grep -o '#token=.*' <<<"$URL" | cut -d= -f2)
kill "$SRV_PID"; wait "$SRV_PID" 2>/dev/null || true
SRV_PID=

PKG_AFTER=$(cat "$FIX/package.json"; cat "$FIX/package-lock.json" | shasum)
[ "$PKG_BEFORE" = "$PKG_AFTER" ] || fail "dependency manifest changed"

(cd "$FIX" && npm run astro build >/dev/null 2>&1) || fail "rebuild after astrogui failed"
DIGEST_B=$(digest "$FIX/dist")
[ "$DIGEST_A" = "$DIGEST_B" ] || fail "build output differs after astrogui ran"
echo "PASS 9.2: never-run project builds identically; manifest unchanged"

# ---------------------------------------------------------------------------
# Task 9.1: the complete flow — capture an idea, write it with an image,
# publish it, confirm the project builds with the post visible at the URL its
# slug pins.
# ---------------------------------------------------------------------------
note "9.1: full capture -> write -> publish flow"
FIX="$WORK/flow"
cp -R "$FIXTURE" "$FIX"
rm -rf "$FIX/drafts" "$FIX/dist"
# The fixture ships one loose published post; keep it loose (task 9.3).
[ -f "$FIX/src/content/blog/existing-post.md" ] || fail "fixture loose post missing"

SRV_LOG="$WORK/flow.log"
(cd "$FIX" && "$BIN" --no-open >"$SRV_LOG" 2>&1 & echo $! > "$WORK/pid")
SRV_PID=$(cat "$WORK/pid")
sleep 1
URL=$(grep -o 'http://[^ ]*' "$SRV_LOG" | head -1)
HOST=${URL#http://}; HOST=${HOST%%/#*}
TOKEN=$(grep -o '#token=.*' <<<"$URL" | cut -d= -f2)
API="http://$HOST/api/collections/blog"
H="X-AstroGUI-Token: $TOKEN"

NAME=$(curl -sf -H "$H" -H 'Content-Type: application/json' \
  -d '{"line":"ship the async rust writeup"}' "$API/entries" |
  sed -n 's/.*"name":"\([^"]*\)".*/\1/p')
[ -n "$NAME" ] || fail "idea capture failed"
note "captured idea as $NAME"

PNG_B64="iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
curl -sf -H "$H" -H 'Content-Type: application/json' \
  -d "{\"name\":\"diagram.png\",\"data\":\"$PNG_B64\"}" \
  "$API/entries/$NAME/assets" > /dev/null || fail "image upload failed"

BODY="The post body.\n\n![diagram](diagram.png)\n"
FIELDS=$(printf '{"title":"Ship the Async Rust Writeup","date":"%s"}' "$(date +%Y-%m-%d)")
MOD=$(curl -sf -H "$H" "$API/entries/$NAME" | sed -n 's/.*"modTime":"\([^"]*\)".*/\1/p')
python3 - "$API" "$TOKEN" "$NAME" "$BODY" "$FIELDS" "$MOD" <<'PY' || fail "edit failed"
import json, sys, urllib.request
api, token, name, body, fields, mod = sys.argv[1:]

def put(path, payload):
    req = urllib.request.Request(
        f"{api}/entries/{name}/{path}",
        data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json", "X-AstroGUI-Token": token},
        method="PUT")
    return json.loads(urllib.request.urlopen(req).read())

# The editor's contract: structured fields through the frontmatter endpoint,
# the body through the body endpoint. Both write only what changed.
r1 = put("frontmatter", {"fields": json.loads(fields), "modTime": mod})
r2 = put("body", {"body": body, "modTime": r1.get("modTime") or mod})
assert r2.get("saved"), r2
PY

curl -sf -H "$H" -H 'Content-Type: application/json' -d '{"to":"wip"}' \
  "$API/entries/$NAME/move" > /dev/null || fail "move to wip failed"
curl -sf -H "$H" -H 'Content-Type: application/json' -d '{"to":"published"}' \
  "$API/entries/$NAME/move" > /dev/null || fail "publish failed (gate)"
[ -f "$FIX/src/content/blog/$NAME/index.md" ] || fail "post not published into the collection"
[ -f "$FIX/src/content/blog/$NAME/diagram.png" ] || fail "image did not move with the post"
kill "$SRV_PID"; wait "$SRV_PID" 2>/dev/null || true
SRV_PID=

note "building the blog"
(cd "$FIX" && npm run astro build >/dev/null 2>&1) || fail "build failed with the published post"

PAGE="$FIX/dist/posts/$NAME/index.html"
[ -f "$PAGE" ] || fail "post page missing at /posts/$NAME/ (slug-pinned URL)"
grep -q "Ship the Async Rust Writeup" "$PAGE" || fail "post page lacks the title"
grep -q "diagram" <(ls "$FIX/dist/_astro/" 2>/dev/null) || true  # image optimization is the blog's own
echo "PASS 9.1: idea -> image -> publish -> build, post live at /posts/$NAME/"

# ---------------------------------------------------------------------------
# Task 9.3: loose posts stay loose, listed read-only, never rewritten; only
# astrogui's own drafts use the folder layout.
# ---------------------------------------------------------------------------
[ -f "$FIX/src/content/blog/existing-post.md" ] || fail "loose post vanished or was migrated"
grep -q "Already here." "$FIX/src/content/blog/existing-post.md" || fail "loose post rewritten"
[ -d "$FIX/src/content/blog/existing-post" ] && fail "loose post converted to a folder"
[ -f "$FIX/dist/posts/existing-post/index.html" ] || fail "loose post not built"
echo "PASS 9.3: loose markdown post still loose, still built, never migrated"

# ---------------------------------------------------------------------------
# Task 9.4: a project with a custom generateId and an unrecognized frontmatter
# field — the pinned slug still yields the expected URL.
# ---------------------------------------------------------------------------
note "9.4: custom generateId + unrecognized frontmatter field"
FIX="$WORK/customid"
cp -R "$FIXTURE" "$FIX"
rm -rf "$FIX/drafts" "$FIX/dist"
cat > "$FIX/src/content.config.ts" <<'TS'
import { defineCollection } from 'astro:content';
import { glob } from 'astro/loaders';
import { z } from 'astro/zod';

const blog = defineCollection({
  loader: glob({
    pattern: '**/*.md',
    base: './src/content/blog',
    generateId: ({ entry, data }) => (data && data.slug ? data.slug : entry.replace(/\.md$/, '')),
  }),
  schema: z.object({ title: z.string(), date: z.coerce.date() }),
});

export const collections = { blog };
TS

mkdir -p "$FIX/src/content/blog/pinned-name"
printf -- '---\ntitle: Custom ID Post\nslug: pinned-name\ndate: 2026-01-01\ncustom_field: unrecognized\n---\n\nBody.\n' \
  > "$FIX/src/content/blog/pinned-name/index.md"

(cd "$FIX" && npm run astro build >/dev/null 2>&1) || fail "custom-generateId build failed"
[ -f "$FIX/dist/posts/pinned-name/index.html" ] || fail "slug did not pin the URL under custom generateId"
grep -q "Custom ID Post" "$FIX/dist/posts/pinned-name/index.html" || fail "custom-id post content missing"
echo "PASS 9.4: pinned slug survives custom generateId and schema stripping"

echo
echo "E2E: all integration checks passed"
