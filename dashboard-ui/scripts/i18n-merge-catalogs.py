# Merge ordered translation arrays into the language packs' `dashboard`
# catalogs (aurorax-store). The dashboard SPA ships no bundled translations:
# installing a language pack translates the whole dashboard chrome via
# GET /sidecar/extensions/locale/catalog.
#
# Workflow:
#   1. scripts/i18n-keys.json          — JSON array of English source keys (sorted)
#   2. scripts/i18n-translations/<code>.json — JSON array of translations,
#      same order and length as the keys array (written by translators)
#   3. this script → <store>/extensions/locale-<code>.extension.json:
#      injects "dashboard" = {key: translation}, bumps "version".
#
# Usage: python3 scripts/i18n-merge-catalogs.py [--store DIR] [code ...]
#   (default store: ../../aurorax-store/extensions, default codes: all)

import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
KEYS = ROOT / "scripts" / "i18n-keys.json"
TRANS_DIR = ROOT / "scripts" / "i18n-translations"
DEFAULT_STORE = ROOT.parent.parent / "aurorax-store" / "extensions"

args = sys.argv[1:]
store_dir = DEFAULT_STORE
if "--store" in args:
    i = args.index("--store")
    try:
        store_dir = Path(args[i + 1])
    except IndexError:
        sys.exit("--store requires a directory argument")
    del args[i : i + 2]

keys = json.loads(KEYS.read_text(encoding="utf-8"))
n = len(keys)

codes = args or sorted(p.stem for p in TRANS_DIR.glob("*.json"))
if not codes:
    sys.exit(f"no translation files in {TRANS_DIR}")
if not store_dir.is_dir():
    sys.exit(f"store extensions dir not found: {store_dir}")

failed = []
for code in codes:
    src = TRANS_DIR / f"{code}.json"
    if not src.exists():
        print(f"MISSING {code}: {src}")
        failed.append(code)
        continue
    vals = json.loads(src.read_text(encoding="utf-8"))
    if not isinstance(vals, list):
        print(f"BAD {code}: expected a JSON array")
        failed.append(code)
        continue
    if len(vals) != n:
        print(f"BAD {code}: {len(vals)} translations, expected {n}")
        failed.append(code)
        continue
    bad = [i for i, v in enumerate(vals) if not isinstance(v, str) or not v.strip()]
    if bad:
        # A few legitimately-empty translations are allowed only where the
        # source is purely symbolic; report them but keep the catalog usable.
        print(f"WARN {code}: {len(bad)} empty/non-string entries (first: {bad[:5]})")
    dashboard = {k: v for k, v in zip(keys, vals) if isinstance(v, str) and v.strip()}

    pack_path = store_dir / f"locale-{code}.extension.json"
    if not pack_path.exists():
        print(f"MISSING pack {code}: {pack_path}")
        failed.append(code)
        continue
    pack = json.loads(pack_path.read_text(encoding="utf-8"))
    changed = pack.get("dashboard") != dashboard
    pack["dashboard"] = dashboard
    if changed:
        # Bump the integer version so installed packs see "update available".
        try:
            pack["version"] = str(int(str(pack.get("version", "1"))) + 1)
        except ValueError:
            pack["version"] = "2"
    pack_path.write_text(
        json.dumps(pack, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    state = f"v{pack['version']}" + ("" if changed else ", unchanged")
    print(f"OK {code}: {len(dashboard)} dashboard entries → {pack_path.name} ({state})")

if failed:
    sys.exit(f"failed: {', '.join(failed)}")
