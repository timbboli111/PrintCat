#!/usr/bin/env bash
# Package PrintCat with its Android PrintService class and metadata.
#
# Fyne 2.6.3 packages a prebuilt classes.dex and only adds assets and the
# manifest. It does not compile project Java sources or Android XML resources.
# This wrapper preserves Fyne's Go/Fyne APK build, then adds the native
# PrintService into classes.dex and replaces the manifest/resources using the
# Android SDK build tools before re-signing the final APK.
set -euo pipefail

readonly ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly APP_ID="dev.printcat.app"
readonly APP_NAME="PrintCat"
readonly PACKAGE_DIR="$ROOT_DIR/cmd/printcat"
readonly JAVA_SOURCE_ROOT="$ROOT_DIR/android"
readonly RESOURCE_DIR="$ROOT_DIR/android/src/main/res"
readonly MANIFEST="$PACKAGE_DIR/AndroidManifest.xml"
readonly OUTPUT_APK="${1:-$ROOT_DIR/PrintCat.apk}"
readonly FYNE_BIN="${FYNE_BIN:-fyne}"
readonly SDK_ROOT="${ANDROID_HOME:?Set ANDROID_HOME to the Android SDK root.}"

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
require_file "$MANIFEST"
require_command "$FYNE_BIN"
require_command javac
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

readonly WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT
readonly FYNE_APK="$WORK_DIR/fyne.apk"
readonly JAVA_CLASSES="$WORK_DIR/java-classes"
readonly DEX_DIR="$WORK_DIR/dex"
readonly COMPILED_RESOURCES="$WORK_DIR/resources.zip"
readonly RESOURCE_APK="$WORK_DIR/resources.apk"
readonly BASE_MANIFEST="$WORK_DIR/base-AndroidManifest.xml"
readonly FINAL_MANIFEST="$WORK_DIR/final-AndroidManifest.xml"
readonly FINAL_RESOURCES="$WORK_DIR/res"
readonly APK_CONTENTS="$WORK_DIR/apk-contents"
readonly UNSIGNED_APK="$WORK_DIR/unsigned.apk"
readonly ALIGNED_APK="$WORK_DIR/aligned.apk"

mkdir -p "$JAVA_CLASSES" "$DEX_DIR" "$APK_CONTENTS"

cp "$MANIFEST" "$FINAL_MANIFEST"
python3 - "$FINAL_MANIFEST" "$BASE_MANIFEST" <<'PY'
from pathlib import Path
import sys
import xml.etree.ElementTree as ET

android = "{http://schemas.android.com/apk/res/android}"
tree = ET.parse(sys.argv[1])
application = tree.getroot().find("application")
for service in list(application.findall("service")):
    if service.get(android + "name") == "com.printcat.app.PrintCatPrintService":
        application.remove(service)
tree.write(sys.argv[2], encoding="utf-8", xml_declaration=True)
PY

# Fyne 2.6.3 only understands the base manifest/resource set. Temporarily
# supply the base manifest, then restore the final source manifest before any
# Android SDK post-processing occurs.
cp "$BASE_MANIFEST" "$MANIFEST"
restore_manifest() {
    cp "$FINAL_MANIFEST" "$MANIFEST"
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
    "$JAVA_SOURCE_ROOT/src/main/java/com/printcat/app/PrintCatPrintService.java"
unzip -q "$FYNE_APK" classes.dex -d "$WORK_DIR/fyne-dex"
"$D8" --min-api 19 --lib "$ANDROID_JAR" --output "$DEX_DIR" \
    "$WORK_DIR/fyne-dex/classes.dex" "$JAVA_CLASSES"

# Fyne adds its icon while building the initial APK. Recreate that resource in
# the final aapt2-linked manifest so adding PrintService resources does not
# change the existing launcher icon.
mkdir -p "$FINAL_RESOURCES/mipmap-xxxhdpi"
unzip -p "$FYNE_APK" 'res/mipmap-*/icon.png' > "$FINAL_RESOURCES/mipmap-xxxhdpi/icon.png"
python3 - "$FINAL_MANIFEST" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
manifest = path.read_text()
manifest = manifest.replace(
    "<application\n",
    '<application\n        android:icon="@mipmap/icon"\n',
    1,
)
path.write_text(manifest)
PY
cp -R "$RESOURCE_DIR/." "$FINAL_RESOURCES/"
"$AAPT2" compile --dir "$FINAL_RESOURCES" -o "$COMPILED_RESOURCES"
"$AAPT2" link --auto-add-overlay --min-sdk-version 19 -I "$ANDROID_JAR" \
    --manifest "$FINAL_MANIFEST" -o "$RESOURCE_APK" "$COMPILED_RESOURCES"

unzip -q "$FYNE_APK" -d "$APK_CONTENTS"
rm -f "$APK_CONTENTS/classes"*.dex "$APK_CONTENTS/AndroidManifest.xml" "$APK_CONTENTS/resources.arsc"
rm -rf "$APK_CONTENTS/res" "$APK_CONTENTS/META-INF"
cp "$DEX_DIR"/classes*.dex "$APK_CONTENTS/"
unzip -q "$RESOURCE_APK" AndroidManifest.xml resources.arsc -d "$APK_CONTENTS"
if unzip -l "$RESOURCE_APK" 'res/*' | grep -q 'res/'; then
    unzip -q "$RESOURCE_APK" 'res/*' -d "$APK_CONTENTS"
fi

(
    cd "$APK_CONTENTS"
    zip -q -r "$UNSIGNED_APK" .
)
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
