# Merge ordered translation arrays into dashboard UI locale catalogs.
#
# Workflow:
#   1. scripts/i18n-keys.json          — JSON array of English source keys (sorted)
#   2. scripts/i18n-translations/<code>.json — JSON array of translations,
#      same order and length as the keys array (written by translators)
#   3. this script → src/lib/i18n/locales/<code>.json as {key: translation}
#
# Usage: python3 scripts/i18n-merge-catalogs.py [code ...]   (default: all)

import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
KEYS = ROOT / "scripts" / "i18n-keys.json"
TRANS_DIR = ROOT / "scripts" / "i18n-translations"
OUT_DIR = ROOT / "src" / "lib" / "i18n" / "locales"

keys = json.loads(KEYS.read_text(encoding="utf-8"))
n = len(keys)

codes = sys.argv[1:] or sorted(p.stem for p in TRANS_DIR.glob("*.json"))
if not codes:
    sys.exit(f"no translation files in {TRANS_DIR}")

OUT_DIR.mkdir(parents=True, exist_ok=True)
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
    catalog = {k: v for k, v in zip(keys, vals) if isinstance(v, str) and v.strip()}
    out = OUT_DIR / f"{code}.json"
    out.write_text(
        json.dumps(catalog, ensure_ascii=False, indent=1, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(f"OK {code}: {len(catalog)} entries → {out.relative_to(ROOT)}")

if failed:
    sys.exit(f"failed: {', '.join(failed)}")
