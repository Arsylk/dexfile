#!/usr/bin/env bash
# Builds every DEX fixture under testdata/dex from the Java sources under testdata/java.
# Idempotent: wipes testdata/build and testdata/dex and regenerates everything.
#
# Requirements: javac (>= 11), python3, Android SDK with build-tools 37.0.0 (d8, dexdump).
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SDK="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-/opt/android-sdk}}"
BT="${BUILD_TOOLS:-$SDK/build-tools/37.0.0}"
D8="$BT/d8"
DEXDUMP="$BT/dexdump"
ANDROID_JAR="${ANDROID_JAR:-$SDK/platforms/android-36/android.jar}"
JAVAC="${JAVAC:-javac}"
PYTHON="${PYTHON:-python3}"

JAVA="$HERE/java"
BUILD="$HERE/build"
DEX="$HERE/dex"
GEN="$HERE/../scripts/gen_large.py"

for tool in "$D8" "$DEXDUMP" "$ANDROID_JAR" "$GEN"; do
  [[ -e "$tool" ]] || { echo "missing: $tool" >&2; exit 1; }
done
command -v "$JAVAC" >/dev/null || { echo "javac not found" >&2; exit 1; }
command -v "$PYTHON" >/dev/null || { echo "python3 not found" >&2; exit 1; }

rm -rf "$BUILD" "$DEX"
mkdir -p "$BUILD" "$DEX/malformed"

# ---------------------------------------------------------------------------
# 1. Generate the large sources and compile every source set.
# ---------------------------------------------------------------------------
"$PYTHON" "$GEN" "$JAVA/main"

JFLAGS=(--release 11 -g -parameters -encoding UTF-8 -nowarn -Xlint:none -proc:none)

compile() { # compile <name> [extra javac args...] -- <source dirs...>
  local name="$1"; shift
  local extra=()
  while [[ "$1" != "--" ]]; do extra+=("$1"); shift; done; shift
  local out="$BUILD/classes-$name"
  mkdir -p "$out"
  local list="$BUILD/sources-$name.txt"
  find "$@" -name '*.java' | sort > "$list"
  echo "javac  [$name] $(wc -l < "$list") files"
  "$JAVAC" "${JFLAGS[@]}" "${extra[@]}" -d "$out" @"$list"
}

compile main -- "$JAVA/main"
compile indy -- "$JAVA/indy"
compile tiny -- "$JAVA/tiny"
compile dex2 -- "$JAVA/dex2"
compile dex1 -cp "$BUILD/classes-dex2" -- "$JAVA/dex1"

# ---------------------------------------------------------------------------
# 2. d8
# ---------------------------------------------------------------------------
run_d8() { # run_d8 <out.dex> <d8 flags...> -- <inputs...>
  local out="$1"; shift
  local flags=()
  while [[ "$1" != "--" ]]; do flags+=("$1"); shift; done; shift
  local tmp="$BUILD/d8-$(basename "$out" .dex)"
  rm -rf "$tmp"; mkdir -p "$tmp"
  echo "d8     [$(basename "$out")] ${flags[*]}"
  "$D8" --lib "$ANDROID_JAR" "${flags[@]}" --output "$tmp" "$@"
  [[ -f "$tmp/classes.dex" ]] || { echo "d8 produced no classes.dex for $out" >&2; exit 1; }
  if [[ -f "$tmp/classes2.dex" ]]; then echo "unexpected multidex output for $out" >&2; exit 1; fi
  mv "$tmp/classes.dex" "$out"
}

classfiles() { find "$@" -name '*.class' | sort; }

mapfile -t MAIN_CLASSES < <(classfiles "$BUILD/classes-main")
mapfile -t INDY_CLASSES < <(classfiles "$BUILD/classes-indy")
mapfile -t TINY_CLASSES < <(classfiles "$BUILD/classes-tiny")
mapfile -t DEX1_CLASSES < <(classfiles "$BUILD/classes-dex1")
mapfile -t DEX2_CLASSES < <(classfiles "$BUILD/classes-dex2")
ALL_CLASSES=("${MAIN_CLASSES[@]}" "${INDY_CLASSES[@]}")

run_d8 "$DEX/v035.dex"    --min-api 21 --debug                  -- "${ALL_CLASSES[@]}"
run_d8 "$DEX/v037.dex"    --min-api 24 --debug                  -- "${ALL_CLASSES[@]}"
run_d8 "$DEX/v038.dex"    --min-api 26 --debug --no-desugaring  -- "${ALL_CLASSES[@]}"
run_d8 "$DEX/v039.dex"    --min-api 28 --debug                  -- "${ALL_CLASSES[@]}"
run_d8 "$DEX/release.dex" --min-api 21 --release                -- "${ALL_CLASSES[@]}"
run_d8 "$DEX/tiny.dex"    --min-api 21 --debug                  -- "${TINY_CLASSES[@]}"

# multidex.apk: classes.dex = main fixtures + Caller + Dup(v1); classes2.dex = Callee + Dup(v2)
run_d8 "$BUILD/classes.dex"  --min-api 21 --debug --classpath "$BUILD/classes-dex2" -- "${MAIN_CLASSES[@]}" "${DEX1_CLASSES[@]}"
run_d8 "$BUILD/classes2.dex" --min-api 21 --debug                                    -- "${DEX2_CLASSES[@]}"

"$PYTHON" - "$BUILD" "$DEX/multidex.apk" <<'PY'
import sys, zipfile, os
build, out = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(out, "w") as z:
    # Placeholder manifest/resources: not parsed by anything, just realistic-looking entries.
    z.writestr(zipfile.ZipInfo("AndroidManifest.xml"), b"\x03\x00\x08\x00" + b"\x00" * 60, zipfile.ZIP_DEFLATED)
    z.write(os.path.join(build, "classes.dex"), "classes.dex", zipfile.ZIP_STORED)
    z.write(os.path.join(build, "classes2.dex"), "classes2.dex", zipfile.ZIP_DEFLATED)
    z.writestr("resources.arsc", b"\x02\x00\x0c\x00" + b"\x00" * 28, zipfile.ZIP_STORED)
    z.writestr("META-INF/junk.txt", b"not a signature, just junk\n", zipfile.ZIP_DEFLATED)
    z.writestr("assets/notes.txt", b"placeholder asset\n", zipfile.ZIP_DEFLATED)
PY
echo "zip    [multidex.apk]"

# ---------------------------------------------------------------------------
# 3. Malformed inputs derived from tiny.dex
# ---------------------------------------------------------------------------
"$PYTHON" - "$DEX/tiny.dex" "$DEX/malformed" <<'PY'
import struct, sys, os
src, out = sys.argv[1], sys.argv[2]
data = bytearray(open(src, "rb").read())

def write(name, b):
    with open(os.path.join(out, name), "wb") as fh:
        fh.write(b)

write("empty.dex", b"")
write("truncated-header.dex", bytes(data[:60]))
write("truncated-mid.dex", bytes(data[: len(data) // 2]))

bad_magic = bytearray(data); bad_magic[0:8] = b"xex\n035\x00"
write("bad-magic.dex", bytes(bad_magic))

huge = bytearray(data); struct.pack_into("<I", huge, 0x38, 0xFFFFFFFF)   # string_ids_size
write("huge-sizes.dex", bytes(huge))

bad_off = bytearray(data)
struct.pack_into("<I", bad_off, 0x34, 0xFFFFFFF0)   # map_off
struct.pack_into("<I", bad_off, 0x64, 0x7FFFFFFF)   # class_defs_off
write("bad-offsets.dex", bytes(bad_off))

# Bonus: header claims a file_size larger than the real file, and a wrong endian tag.
big_size = bytearray(data); struct.pack_into("<I", big_size, 0x20, len(data) * 16)
write("wrong-file-size.dex", bytes(big_size))
endian = bytearray(data); struct.pack_into("<I", endian, 0x28, 0x78563412)
write("bad-endian-tag.dex", bytes(endian))
PY
echo "python [malformed/]"

# ---------------------------------------------------------------------------
# 4. dexdump oracles
# ---------------------------------------------------------------------------
for f in "$DEX"/*.dex; do
  base="${f%.dex}"
  "$DEXDUMP" -d "$f" > "$base.dexdump.txt"
  "$DEXDUMP" -f "$f" > "$base.dexdump-f.txt"
done
mkdir -p "$BUILD/apk"
"$DEXDUMP" -d "$BUILD/classes.dex"  > "$DEX/multidex.classes.dexdump.txt"
"$DEXDUMP" -f "$BUILD/classes.dex"  > "$DEX/multidex.classes.dexdump-f.txt"
"$DEXDUMP" -d "$BUILD/classes2.dex" > "$DEX/multidex.classes2.dexdump.txt"
"$DEXDUMP" -f "$BUILD/classes2.dex" > "$DEX/multidex.classes2.dexdump-f.txt"
echo "dexdump oracles written"

# ---------------------------------------------------------------------------
# 5. Summary
# ---------------------------------------------------------------------------
for f in "$DEX"/*.dex "$BUILD"/classes.dex "$BUILD"/classes2.dex; do
  printf '%-14s magic=%-8q size=%s\n' "$(basename "$f")" "$(head -c 8 "$f" | tr -d '\0')" "$(stat -c %s "$f")"
done
echo "done: $DEX"
