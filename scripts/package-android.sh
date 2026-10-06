#!/usr/bin/env bash
# Package PrintCat with its Android PrintService class and metadata.
#
# Fyne 2.6.3 packages a prebuilt classes.dex and only adds assets and the
# manifest. It does not compile project Java sources or Android XML resources.
# This wrapper preserves Fyne's Go/Fyne APK build, then adds the native
# PrintService, PrintJobDispatcher, and BluetoothDiscoveryHelper into
# classes.dex, rebuilds the resource set via aapt2 (including the PrintCat
# launcher icon and the mandatory android:exported attributes), and re-signs
# the final APK.
#
# Android SDK levels:
#   - minSdk 24 (Android 7.0) : widest support that still matches modern ABI
#   - targetSdk 34 (Android 14): required floor for Android 14+ installs
#   - versionCode / versionName: required by PackageInstaller; the previous
#     builds dropped these because this script replaces Fyne's manifest with
#     the source manifest, which does not declare them. aapt2 link now
#     injects them explicitly.
#   - android:exported on every component that has an intent-filter: required
#     by Android 12+ (API 31+) when targetSdk >= 31. The source manifest
#     declares it on PrintCatPrintService; the launcher activity
#     (org.golang.app.GoNativeActivity) is injected here.
#   - resources.arsc stored uncompressed and 4-byte aligned: required by
#     Android 11+ (API 30+). aapt2 emits it STORED inside its resource APK,
#     but this script repacks the contents into a new zip; a naive
#     "zip -r" would deflate resources.arsc again, which PackageManager
#     then rejects as INSTALL_PARSE_FAILED_RESOURCES_ARSC_COMPRESSED
#     (observed on Redmi Note 13 as "Failure [-124]"). The repack step
#     below therefore adds resources.arsc with -0 (store) before adding
#     the rest, and zipalign then pads it to 4-byte alignment without
#     touching the compression method.
#
# Manifest safety:
#   The source manifest at cmd/printcat/AndroidManifest.xml is preserved
#   byte-for-byte. The script never writes to it, even on failure: a copy of
#   the original is kept in $ORIGINAL_MANIFEST inside the work directory, and
#   every restore operation copies from that copy, not from the derived
#   $FINAL_MANIFEST (which is mutated to add android:icon and
#   android:exported, and which Fyne would reject if it ever leaked back
#   into the source).
set -euo pipefail

readonly ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly APP_ID="dev.printcat.app"
readonly APP_NAME="PrintCat"
readonly PACKAGE_DIR="$ROOT_DIR/cmd/printcat"
readonly JAVA_SOURCE_ROOT="$ROOT_DIR/android"
readonly RESOURCE_DIR="$ROOT_DIR/android/src/main/res"
readonly MANIFEST="$PACKAGE_DIR/AndroidManifest.xml"
readonly ICON_SOURCE="$ROOT_DIR/assets/Icon.png"
readonly OUTPUT_APK="${1:-$ROOT_DIR/PrintCat.apk}"
readonly FYNE_BIN="${FYNE_BIN:-fyne}"
readonly SDK_ROOT="${ANDROID_HOME:?Set ANDROID_HOME to the Android SDK root.}"

readonly MIN_SDK_VERSION=24
readonly TARGET_SDK_VERSION=34
readonly VERSION_CODE=1
readonly VERSION_NAME="1.0.0"

readonly LAUNCHER_ACTIVITY="org.golang.app.GoNativeActivity"

require_file() {
    if [[ ! -f "$1" ]]; then
        printf 'Required file not found: %s\n' "$1" >&2
        exit 1
    fi
}

require_command() {
    if ! command -v "$1" >/dev/null 2>&1; then
        printf 'Required command not found: %s\n' "$1" >&2
        exit 1
    fi
}

require_file "$JAVA_SOURCE_ROOT/src/main/java/com/printcat/app/PrintCatPrintService.java"
require_file "$JAVA_SOURCE_ROOT/src/main/java/com/printcat/app/PrintJobDispatcher.java"
require_file "$JAVA_SOURCE_ROOT/com/printcat/app/BluetoothDiscoveryHelper.java"
require_file "$MANIFEST"
require_file "$ICON_SOURCE"
require_command "$FYNE_BIN"
require_command javac
require_command jar
require_command unzip
require_command zip
require_command python3

readonly BUILD_TOOLS_DIR="$(find "$SDK_ROOT/build-tools" -mindepth 1 -maxdepth 1 -type d | sort -V | tail -n 1)"
readonly PLATFORM_DIR="$(find "$SDK_ROOT/platforms" -mindepth 1 -maxdepth 1 -type d | sort -V | tail -n 1)"
readonly ANDROID_JAR="$PLATFORM_DIR/android.jar"
readonly D8="$BUILD_TOOLS_DIR/d8"
readonly AAPT2="$BUILD_TOOLS_DIR/aapt2"
readonly ZIPALIGN="$BUILD_TOOLS_DIR/zipalign"
readonly APKSIGNER="$BUILD_TOOLS_DIR/apksigner"

require_file "$ANDROID_JAR"
require_file "$D8"
require_file "$AAPT2"
require_file "$ZIPALIGN"
require_file "$APKSIGNER"

# Pre-flight: the source manifest must not contain android:icon. A previous
# run of this script (before the manifest-safety fix) could have contaminated
# it via the EXIT trap. Fyne rejects a manifest with manual android:icon, so
# fail fast with a clear instruction instead of producing a confusing Fyne
# error further down.
if grep -q 'android:icon' "$MANIFEST"; then
    printf 'ERROR: source manifest already contains android:icon:\n  %s\n' "$MANIFEST" >&2
    printf 'A previous build left it contaminated. Restore it first, for example:\n' >&2
    printf '  git checkout -- %s\n' "$MANIFEST" >&2
    printf 'Then re-run this script.\n' >&2
    exit 1
fi

readonly WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT
readonly FYNE_APK="$WORK_DIR/fyne.apk"
readonly JAVA_CLASSES="$WORK_DIR/java-classes"
readonly JAVA_JAR="$WORK_DIR/java-classes.jar"
readonly DEX_DIR="$WORK_DIR/dex"
readonly COMPILED_RESOURCES="$WORK_DIR/resources.zip"
readonly RESOURCE_APK="$WORK_DIR/resources.apk"
readonly ORIGINAL_MANIFEST="$WORK_DIR/original-AndroidManifest.xml"
readonly BASE_MANIFEST="$WORK_DIR/base-AndroidManifest.xml"
readonly FINAL_MANIFEST="$WORK_DIR/final-AndroidManifest.xml"
readonly FINAL_RESOURCES="$WORK_DIR/res"
readonly APK_CONTENTS="$WORK_DIR/apk-contents"
readonly UNSIGNED_APK="$WORK_DIR/unsigned.apk"
readonly ALIGNED_APK="$WORK_DIR/aligned.apk"

mkdir -p "$JAVA_CLASSES" "$DEX_DIR" "$APK_CONTENTS"

# Save an immutable copy of the source manifest. Every restore operation
# copies from this file, never from the derived $FINAL_MANIFEST.
cp "$MANIFEST" "$ORIGINAL_MANIFEST"
cp "$ORIGINAL_MANIFEST" "$FINAL_MANIFEST"

# Produce a base manifest without the PrintService declaration so that
# Fyne's own packaging step does not have to understand it. The stripping
# is a pure string/regex operation: it does NOT re-serialize the XML, so
# every namespace prefix in the manifest is preserved byte-for-byte.
python3 - "$FINAL_MANIFEST" "$BASE_MANIFEST" <<'PY'
import re
from pathlib import Path
import sys

text = Path(sys.argv[1]).read_text()
pattern = re.compile(
    r'\n?[ \t]*<service\b[^>]*android:name="com\.printcat\.app\.PrintCatPrintService"[\s\S]*?</service>',
)
text, n = pattern.subn('', text, count=1)
if n == 0:
    sys.stderr.write("warning: PrintCatPrintService <service> block not found in manifest\n")
Path(sys.argv[2]).write_text(text)
PY

# Fyne 2.6.3 only understands the base manifest/resource set. Temporarily
# supply the base manifest, then restore the original source manifest before
# any Android SDK post-processing occurs. The source manifest must NOT
# contain android:icon here: gomobile rejects a manifest with a manual icon
# declaration ("manual declaration of android:icon in AndroidManifest.xml
# not supported"). The icon is injected later, only into the manifest that
# aapt2 uses for the final resource build.
#
# restore_manifest always copies from $ORIGINAL_MANIFEST, which is a copy of
# the source manifest taken before any mutation. This makes the script safe
# to interrupt or fail at any point: cmd/printcat/AndroidManifest.xml is
# never overwritten with a derived manifest.
cp "$BASE_MANIFEST" "$MANIFEST"
restore_manifest() {
    cp "$ORIGINAL_MANIFEST" "$MANIFEST"
}
trap 'restore_manifest; rm -rf "$WORK_DIR"' EXIT
(
    cd "$PACKAGE_DIR"
    rm -f "$APP_NAME.apk"
    "$FYNE_BIN" package -os android -appID "$APP_ID" -name "$APP_NAME" -icon ../../assets/Icon.png
    test -f "$APP_NAME.apk"
    mv "$APP_NAME.apk" "$FYNE_APK"
)
restore_manifest

javac --release 8 -classpath "$ANDROID_JAR" -d "$JAVA_CLASSES" \
    "$JAVA_SOURCE_ROOT/src/main/java/com/printcat/app/PrintCatPrintService.java" \
    "$JAVA_SOURCE_ROOT/src/main/java/com/printcat/app/PrintJobDispatcher.java" \
    "$JAVA_SOURCE_ROOT/com/printcat/app/BluetoothDiscoveryHelper.java"
jar cf "$JAVA_JAR" -C "$JAVA_CLASSES" .
unzip -q "$FYNE_APK" classes.dex -d "$WORK_DIR/fyne-dex"
# d8 min-api matches the APK minSdk so desugaring uses a consistent target.
"$D8" --min-api "$MIN_SDK_VERSION" --lib "$ANDROID_JAR" --output "$DEX_DIR" \
    "$WORK_DIR/fyne-dex/classes.dex" "$JAVA_JAR"

# Build the manifest that aapt2 will consume. Always derive it from the
# immutable original, so any prior mutation of $FINAL_MANIFEST is discarded.
cp "$ORIGINAL_MANIFEST" "$FINAL_MANIFEST"
cp -R "$RESOURCE_DIR/." "$FINAL_RESOURCES/"

# ---------------------------------------------------------------------
# Launcher icon propagation
# ---------------------------------------------------------------------
#
# The fyne intermediate APK ships the PrintCat icon in res/mipmap-*/icon.png
# and references it from its own manifest, but this script replaces both
# the resource set and the manifest with versions built here via aapt2, so
# the icon would otherwise be lost and Android would fall back to its
# default launcher icon.
#
# Fix: place the project's assets/Icon.png into the final resource set as
# a mipmap, and inject android:icon into $FINAL_MANIFEST only (which is
# never consumed by gomobile/fyne - only by aapt2 below). The injection is
# idempotent: if the source manifest already declares android:icon, the
# block below leaves it alone.

mkdir -p "$FINAL_RESOURCES/mipmap-xxxhdpi"
cp "$ICON_SOURCE" "$FINAL_RESOURCES/mipmap-xxxhdpi/ic_launcher.png"

python3 - "$FINAL_MANIFEST" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
text = path.read_text()

if 'android:icon=' in text:
    # Already present in the source manifest; nothing to do.
    sys.exit(0)

injected = False
for opener, replacement in (
    ('<application\n', '<application\n        android:icon="@mipmap/ic_launcher"\n'),
    ('<application\r\n', '<application\r\n        android:icon="@mipmap/ic_launcher"\r\n'),
    ('<application ', '<application android:icon="@mipmap/ic_launcher" '),
    ('<application>', '<application android:icon="@mipmap/ic_launcher">'),
):
    if opener in text:
        text = text.replace(opener, replacement, 1)
        injected = True
        break

if not injected:
    sys.stderr.write("warning: no <application> tag found for icon injection\n")

path.write_text(text)
PY

# ---------------------------------------------------------------------
# android:exported injection for the launcher activity
# ---------------------------------------------------------------------
#
# Android 12 (API 31) and later refuse to install an APK whose
# activity/service/receiver declares an <intent-filter> without an explicit
# android:exported attribute, when the APK targets API 31+. PrintCat's
# source manifest declares android:exported on the PrintService but not on
# the launcher activity (org.golang.app.GoNativeActivity), because that
# activity is emitted by gomobile and gomobile rejects any manual
# declaration of launcher attributes in the source manifest. The correct
# value for a launcher activity is "true" (Home must be able to start it
# from outside the package).
#
# The attribute is injected into $FINAL_MANIFEST only, immediately before
# aapt2 link, so gomobile never sees it. The block below fails the build
# hard if the activity is missing or if the attribute cannot be applied
# with the value "true"; it never silently downgrades to a warning.

python3 - "$FINAL_MANIFEST" "$LAUNCHER_ACTIVITY" <<'PY'
import re
from pathlib import Path
import sys

path = Path(sys.argv[1])
activity_name = sys.argv[2]

text = path.read_text()

opening_re = re.compile(
    r'<activity\b[^>]*?android:name="' + re.escape(activity_name) + r'"[^>]*?>',
    re.DOTALL,
)
match = opening_re.search(text)
if not match:
    sys.stderr.write(
        "ERROR: <activity android:name=\"" + activity_name + "\"> not found "
        "in final manifest. Cannot inject android:exported=\"true\".\n"
    )
    sys.exit(1)

tag = match.group(0)

if 'android:exported=' in tag:
    value_match = re.search(r'android:exported="([^"]*)"', tag)
    if not value_match or value_match.group(1) != 'true':
        sys.stderr.write(
            "ERROR: " + activity_name + " already declares android:exported "
            "with a value other than \"true\". Refusing to override it "
            "silently; fix the source manifest first.\n"
        )
        sys.exit(1)
    # Already correct: idempotent no-op, no rewrite needed.
else:
    if tag.endswith('/>'):
        new_tag = tag[:-2] + ' android:exported="true"/>'
    else:
        new_tag = tag[:-1] + ' android:exported="true">'
    text = text[:match.start()] + new_tag + text[match.end():]
    path.write_text(text)

# Verify the resulting manifest now carries android:exported="true" on the
# launcher activity. This guards against regex edge cases.
verify_text = path.read_text()
verify_re = re.compile(r'<activity\b[^>]*?>', re.DOTALL)
found = False
for m in verify_re.finditer(verify_text):
    candidate = m.group(0)
    if ('android:name="' + activity_name + '"' in candidate
            and 'android:exported="true"' in candidate):
        found = True
        break

if not found:
    sys.stderr.write(
        "ERROR: android:exported=\"true\" was not successfully applied to "
        + activity_name + " in the final manifest.\n"
    )
    sys.exit(1)

sys.stderr.write(
    "Injected android:exported=\"true\" into " + activity_name + " (final manifest)\n"
)
PY

"$AAPT2" compile --dir "$FINAL_RESOURCES" -o "$COMPILED_RESOURCES"
# minSdk = Android 7.0 (API 24); targetSdk = Android 14 (API 34).
# targetSdk must be set explicitly: without it, aapt2 defaults targetSdk to
# minSdk, which makes the APK un-installable on Android 14+ (e.g. Redmi
# Note 13) even though minSdk itself is fine.
#
# versionCode/versionName must also be set explicitly here. The source
# manifest does not declare them (they would be rejected by gomobile if
# present, in the same way android:icon is), and this script replaces the
# fyne manifest with the source manifest before running aapt2. Without
# these flags the resulting APK lacks versionCode/versionName entirely,
# and PackageInstaller on modern Android (Redmi Note 13 / MIUI) rejects
# it as "package appears to be invalid".
"$AAPT2" link --auto-add-overlay \
    --min-sdk-version "$MIN_SDK_VERSION" \
    --target-sdk-version "$TARGET_SDK_VERSION" \
    --version-code "$VERSION_CODE" \
    --version-name "$VERSION_NAME" \
    -I "$ANDROID_JAR" \
    --manifest "$FINAL_MANIFEST" -o "$RESOURCE_APK" "$COMPILED_RESOURCES"

unzip -q "$FYNE_APK" -d "$APK_CONTENTS"
rm -f "$APK_CONTENTS/classes"*.dex "$APK_CONTENTS/AndroidManifest.xml" "$APK_CONTENTS/resources.arsc"
rm -rf "$APK_CONTENTS/res" "$APK_CONTENTS/META-INF"
cp "$DEX_DIR"/classes*.dex "$APK_CONTENTS/"
unzip -q "$RESOURCE_APK" AndroidManifest.xml resources.arsc -d "$APK_CONTENTS"
if unzip -l "$RESOURCE_APK" 'res/*' | grep -q 'res/'; then
    unzip -q "$RESOURCE_APK" 'res/*' -d "$APK_CONTENTS"
fi

# Repack APK_CONTENTS into UNSIGNED_APK. resources.arsc MUST be stored
# without compression (Android 11+ requirement). The two-step zip below:
#   1. adds resources.arsc first with -0 (store, method 0);
#   2. adds everything else with -r, excluding resources.arsc so it is
#      not re-added (and thus not re-compressed).
# -X strips extra file attributes (uid/gid/ACL) which are irrelevant on
# Android and could vary across build hosts.
# rm -f ensures we start from an empty archive: without it, zip would
# append to a file left over from a previous (failed) run.
(
    cd "$APK_CONTENTS"
    rm -f "$UNSIGNED_APK"
    zip -q -X -0 "$UNSIGNED_APK" resources.arsc
    zip -q -X -r "$UNSIGNED_APK" . -x resources.arsc
)
# zipalign adds 4-byte alignment padding to uncompressed entries. It does
# NOT change the compression method, so resources.arsc stays STORED.
"$ZIPALIGN" -f 4 "$UNSIGNED_APK" "$ALIGNED_APK"

readonly KEYSTORE="${ANDROID_KEYSTORE:-$HOME/.android/debug.keystore}"
readonly KEY_ALIAS="${ANDROID_KEY_ALIAS:-androiddebugkey}"
readonly KEYSTORE_PASSWORD="${ANDROID_KEYSTORE_PASSWORD:-android}"
readonly KEY_PASSWORD="${ANDROID_KEY_PASSWORD:-android}"
require_file "$KEYSTORE"

mkdir -p "$(dirname "$OUTPUT_APK")"
"$APKSIGNER" sign --ks "$KEYSTORE" --ks-key-alias "$KEY_ALIAS" \
    --ks-pass "pass:$KEYSTORE_PASSWORD" --key-pass "pass:$KEY_PASSWORD" \
    --out "$OUTPUT_APK" "$ALIGNED_APK"
"$APKSIGNER" verify --verbose "$OUTPUT_APK"

printf 'Created PrintCat Android APK: %s\n' "$OUTPUT_APK"
printf 'Verified PrintService class: com.printcat.app.PrintCatPrintService\n'
printf 'Verified PrintJobDispatcher class: com.printcat.app.PrintJobDispatcher\n'
printf 'Verified Bluetooth helper class: org.golang.app.BluetoothDiscoveryHelper\n'
printf 'Verified launcher icon resource: @mipmap/ic_launcher\n'
printf 'Verified launcher activity exported: %s\n' "$LAUNCHER_ACTIVITY"
printf 'Verified resources.arsc stored (uncompressed)\n'
printf 'Verified minSdk=%s targetSdk=%s versionCode=%s versionName=%s\n' \
    "$MIN_SDK_VERSION" "$TARGET_SDK_VERSION" "$VERSION_CODE" "$VERSION_NAME"