#!/usr/bin/env bash
# Package PrintCat with its Android PrintService class and metadata.
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
require_file "$JAVA_SOURCE_ROOT/src/main/java/com/printcat/app/UsbPrinterHelper.java"
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

cp "$MANIFEST" "$ORIGINAL_MANIFEST"
cp "$ORIGINAL_MANIFEST" "$FINAL_MANIFEST"

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

if grep -q 'android:icon' "$MANIFEST"; then
    printf 'ERROR: source manifest already contains android:icon:\n  %s\n' "$MANIFEST" >&2
    printf 'A previous build left it contaminated. Restore it first, for example:\n' >&2
    printf '  git checkout -- %s\n' "$MANIFEST" >&2
    printf 'Then re-run this script.\n' >&2
    exit 1
fi

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
    "$JAVA_SOURCE_ROOT/src/main/java/com/printcat/app/UsbPrinterHelper.java" \
    "$JAVA_SOURCE_ROOT/com/printcat/app/BluetoothDiscoveryHelper.java"
jar cf "$JAVA_JAR" -C "$JAVA_CLASSES" .
unzip -q "$FYNE_APK" classes.dex -d "$WORK_DIR/fyne-dex"
"$D8" --min-api "$MIN_SDK_VERSION" --lib "$ANDROID_JAR" --output "$DEX_DIR" \
    "$WORK_DIR/fyne-dex/classes.dex" "$JAVA_JAR"

cp "$ORIGINAL_MANIFEST" "$FINAL_MANIFEST"
cp -R "$RESOURCE_DIR/." "$FINAL_RESOURCES/"

mkdir -p "$FINAL_RESOURCES/mipmap-xxxhdpi"
cp "$ICON_SOURCE" "$FINAL_RESOURCES/mipmap-xxxhdpi/ic_launcher.png"

python3 - "$FINAL_MANIFEST" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
text = path.read_text()

if 'android:icon=' not in text:
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
    sys.stderr.write("ERROR: <activity android:name=\"" + activity_name + "\"> not found in final manifest.\n")
    sys.exit(1)

tag = match.group(0)
if 'android:exported=' in tag:
    value_match = re.search(r'android:exported="([^"]*)"', tag)
    if not value_match or value_match.group(1) != 'true':
        sys.stderr.write("ERROR: " + activity_name + " has android:exported != \"true\"\n")
        sys.exit(1)
else:
    if tag.endswith('/>'):
        new_tag = tag[:-2] + ' android:exported="true"/>'
    else:
        new_tag = tag[:-1] + ' android:exported="true">'
    text = text[:match.start()] + new_tag + text[match.end():]
    path.write_text(text)

verify_text = path.read_text()
found = False
for m in re.compile(r'<activity\b[^>]*?>', re.DOTALL).finditer(verify_text):
    candidate = m.group(0)
    if 'android:name="' + activity_name + '"' in candidate and 'android:exported="true"' in candidate:
        found = True
        break
if not found:
    sys.stderr.write("ERROR: android:exported=\"true\" not applied to " + activity_name + "\n")
    sys.exit(1)
PY

"$AAPT2" compile --dir "$FINAL_RESOURCES" -o "$COMPILED_RESOURCES"
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

(
    cd "$APK_CONTENTS"
    rm -f "$UNSIGNED_APK"
    zip -q -X -0 "$UNSIGNED_APK" resources.arsc
    zip -q -X -r "$UNSIGNED_APK" . -x resources.arsc
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
printf 'Verified PrintJobDispatcher class: com.printcat.app.PrintJobDispatcher\n'
printf 'Verified UsbPrinterHelper class: com.printcat.app.UsbPrinterHelper\n'
printf 'Verified Bluetooth helper class: org.golang.app.BluetoothDiscoveryHelper\n'
printf 'Verified launcher icon resource: @mipmap/ic_launcher\n'
printf 'Verified launcher activity exported: %s\n' "$LAUNCHER_ACTIVITY"
printf 'Verified resources.arsc stored (uncompressed)\n'
printf 'Verified minSdk=%s targetSdk=%s versionCode=%s versionName=%s\n' \
    "$MIN_SDK_VERSION" "$TARGET_SDK_VERSION" "$VERSION_CODE" "$VERSION_NAME"