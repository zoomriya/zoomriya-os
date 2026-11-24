#!/bin/sh
# Download the data the town list is built from (docs/weather.md W-1, W-4),
# before a release: packages/zoomriya-weather/build reads these files and
# never downloads anything itself.
#
#   build/geonames    GeoNames' cities1000.zip (every place over 1,000
#                     people) and admin1CodesASCII.txt (provinces and states),
#                     from download.geonames.org. CC BY 4.0.
#   build/meteoalarm  EUMETNET – MeteoAlarm's areas, the newest
#                     MeteoAlarm_Geocodes_<date>.json in its documents
#                     repository (gitlab.com/meteoalarm-pm-group/documents),
#                     to find which areas each European town lies in. CC BY 4.0.
#
# Each folder gets "downloaded" (the date) and "source" (what was fetched),
# which the package's credits name. A download is checked (a zip with the
# towns in it, enough areas) and replaces the old files only once whole; a
# failed one leaves the last good files.
#
# GeoNames changes daily and MeteoAlarm a few times a year: run it before
# each release, then scripts/build-packages.sh.
#
# Usage: scripts/download-towns.sh [geonames|meteoalarm]   (both by default)
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
what=${1:-all}
agent="ZOOMRIYA-OS-build (https://zoomriya.com)"

fetch() { curl -fsSL --retry 3 -A "$agent" -o "$2" "$1"; }

geonames() {
    dir="$root/build/geonames"
    work=$(mktemp -d)
    trap 'rm -rf "$work"' EXIT
    echo "GeoNames: cities1000 and admin1 codes"
    fetch https://download.geonames.org/export/dump/cities1000.zip "$work/cities1000.zip"
    fetch https://download.geonames.org/export/dump/admin1CodesASCII.txt "$work/admin1CodesASCII.txt"
    python3 -I - "$work" <<'PY'
import sys, zipfile
work = sys.argv[1]
with zipfile.ZipFile(f"{work}/cities1000.zip") as z:
    if z.testzip() is not None or "cities1000.txt" not in z.namelist():
        raise SystemExit("cities1000.zip isn't whole")
    towns = sum(1 for _ in z.open("cities1000.txt"))
if towns < 100000:
    raise SystemExit(f"cities1000 looks cut short: {towns} towns")
if sum(1 for _ in open(f"{work}/admin1CodesASCII.txt", encoding="utf-8")) < 3000:
    raise SystemExit("admin1CodesASCII looks cut short")
print(f"  {towns} towns")
PY
    date -u +%F > "$work/downloaded"
    echo "cities1000.zip, admin1CodesASCII.txt (download.geonames.org)" > "$work/source"
    mkdir -p "$dir"
    mv -f "$work"/* "$dir/"
    rm -rf "$work"
    trap - EXIT
}

meteoalarm() {
    dir="$root/build/meteoalarm"
    work=$(mktemp -d)
    trap 'rm -rf "$work"' EXIT
    repo=https://gitlab.com/api/v4/projects/meteoalarm-pm-group%2Fdocuments
    fetch "$repo/repository/tree?per_page=100" "$work/tree.json"
    newest=$(python3 -I -c 'import json,sys; print(max(f["name"] for f in json.load(open(sys.argv[1])) if f["name"].startswith("MeteoAlarm_Geocodes_2") and f["name"].endswith(".json")))' "$work/tree.json")
    echo "MeteoAlarm: $newest"
    fetch "https://gitlab.com/meteoalarm-pm-group/documents/-/raw/master/$newest" "$work/geocodes.json"
    python3 -I - "$work/geocodes.json" <<'PY'
import json, sys
areas = json.load(open(sys.argv[1], encoding="utf-8"))["features"]
if len(areas) < 1000 or any(a["properties"].get("type") != "EMMA_ID" for a in areas):
    raise SystemExit(f"MeteoAlarm's areas look wrong: {len(areas)}")
print(f"  {len(areas)} areas in {len({a['properties']['country'] for a in areas})} countries")
PY
    rm -f "$work/tree.json"
    date -u +%F > "$work/downloaded"
    echo "$newest" > "$work/source"
    mkdir -p "$dir"
    mv -f "$work"/* "$dir/"
    rm -rf "$work"
    trap - EXIT
}

case "$what" in
    all) geonames; meteoalarm ;;
    geonames) geonames ;;
    meteoalarm) meteoalarm ;;
    *) echo "Unknown: $what (geonames or meteoalarm)" >&2; exit 1 ;;
esac
