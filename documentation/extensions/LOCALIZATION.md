# Extension localization (i18n)

Extensions can present themselves in the operator's language. Localization
covers the translatable surface of a manifest — never settings values, headers
or code:

| Area | Translatable keys |
|------|-------------------|
| Identity | `name`, `tagline`, `description` |
| UI basics | `ui.help`, `ui.logo_text` |
| Settings form | `ui.fields[].label`, `ui.fields[].description` |
| Navigation | `ui.nav[].label` |
| Banners | `ui.banners[].message`, `ui.banners[].link_label` |
| Widgets | `ui.widgets[].title`, `ui.widgets[].body` |
| Pages | `ui.pages[].title`, `ui.pages[].summary`, `ui.pages[].blocks[].text` / `.items` |
| Settings tabs | `ui.settings_tabs[].label`, `ui.settings_tabs[].blocks[].text` / `.items` |

Translations come from two sources, resolved per request:

1. **Embedded** — the manifest itself ships `i18n.locales.<code>`.
2. **Language packs** — a `type: "localization"` extension carries a
   `catalog` keyed by target extension id. **Applied packs override embedded
   translations** for their locale.

## Embedded translations

```json
{
  "id": "my-extension",
  "name": "My Extension",
  "ui": { "help": "How to use it" },
  "i18n": {
    "locales": {
      "ru": {
        "name": "Моё расширение",
        "tagline": "Описание",
        "ui": {
          "help": "Как этим пользоваться",
          "fields": { "region": { "label": "Регион", "description": "Куда подключаться" } },
          "nav": { "docs": "Документация" },
          "pages": { "about": { "title": "О расширении", "blocks": { "0": { "text": "Абзац" } } } },
          "settings_tabs": { "auth": { "label": "Авторизация" } }
        }
      },
      "pt-br": { "name": "Minha extensão" }
    }
  }
}
```

Field maps are keyed by the entry ids of the source manifest (`fields` by
field key, `nav`/`banners`/`widgets`/`pages`/`settings_tabs` by their `id`).
Page and tab blocks are keyed by their **decimal index** in the source array,
so inserting a block above them shifts the indexes and the stale translation
stops matching instead of landing on the wrong paragraph. A block translation
carries `text` (heading/text/code blocks) and/or `items` (a list block,
replaced as a whole array):

```json
"blocks": {
  "0": { "text": "Как это работает" },
  "7": { "items": ["Шаг первый", "Шаг второй"] }
}
```

Empty strings mean "not translated here": the source text (or a
lower-precedence translation) stays.

## Language packs (`type: "localization"`)

A language pack translates *other* extensions and (optionally) the whole
dashboard UI. It pins one locale and carries a catalog keyed by target
extension id, a `dashboard` catalog of dashboard chrome strings, or both:

```json
{
  "schema": 1,
  "id": "locale-ru",
  "name": "Русский язык",
  "type": "localization",
  "locale": "ru",
  "catalog": {
    "timestamped": {
      "name": "ExampleClient",
      "tagline": "Шлюз ExampleClient",
      "ui": {
        "help": "Справка на русском",
        "settings_tabs": { "auth": { "label": "Авторизация" } }
      }
    },
    "vpn-support": { "ui": { "fields": { "mode": { "label": "Режим" } } } }
  },
  "dashboard": {
    "Save": "Сохранить",
    "Delete": "Удалить"
  }
}
```

Validation requires `locale` and at least one of `catalog` / `dashboard`;
every catalog entry must carry at least one non-empty string or a `ui`
object, and every `dashboard` entry a non-empty translation.

**Language packs are install/uninstall only — there is no apply toggle:**

- Importing (or installing from a store) a pack **activates it immediately**.
- **Unapply is rejected** (400): delete the extension to remove its
  translations.
- A localization pack **never touches sidecar settings**, Session Hub headers,
  auth wiring or client profiles.
- **Several packs may be installed at once** (unlike themes, which are
  one-at-a-time). When several packs translate the same string, the one with
  the higher `order` wins (ties broken by id).

## Choosing the language

The active locale is a persisted gateway setting:

```http
GET  /admin/api/v1/sidecar/extensions/locale
     → { "locale": "ru", "available": ["de", "ru"] }

PUT  /admin/api/v1/sidecar/extensions/locale
     { "locale": "ru" }        → { "status": "ok", "locale": "ru", … }
     { "locale": "" }          → reset to the source language
```

It is stored in `configs/extension-locale.json` (path override:
`AURORA_EXTENSION_LOCALE_PATH`). The dashboard exposes it as a language
selector in **Settings → Extensions**.

List endpoints accept `?locale=<code>` as a per-request override (valid codes
only; an unknown locale simply keeps the source text).

### Resolution rules

For each extension, the locale code is picked from what that extension
actually has (embedded locales ∪ installed pack locales for it):

1. exact code (`pt-BR` → `pt-BR`),
2. bare base language (`pt-BR` → `pt`),
3. single region variant of the same language (`pt-PT` → `pt`, if that is the
   only one),
4. `zh` prefers `zh-cn` when several `zh-*` translations exist,
5. otherwise the **source text** (no guessing between competing variants
   such as `sr-cyrl` vs `sr-latn`).

Then embedded translations are layered first and installed packs on top
(pack wins where both translate the same key).

### Editor safety

`GET /sidecar/extensions/:id`, the JSON editor and exports always return the
**stored document verbatim** — localization is applied only to
`GET /sidecar/extensions` and `GET /sidecar/extensions/ui`. Editing what you
see is never corrupted by the active language.

## Dashboard UI localization

The dashboard UI localizes itself from the **same persisted locale** (there is
one language switch for the whole product). English source strings double as
translation keys, so any string missing from a catalog simply falls back to
English:

```ts
import { t } from "@/lib/i18n";

t("Save")                     // "Сохранить" for locale=ru, "Save" otherwise
t("Delete {count} items", { count: 3 })
```

**Catalogs are not bundled into the SPA.** They ship inside installed
language packs (the pack's `dashboard` map) and are served by the gateway:

```http
GET /admin/api/v1/sidecar/extensions/locale/catalog[?locale=<code>]
     → { "locale": "ru", "catalog": { "Save": "Сохранить", … } }
```

All installed packs whose locale resolves to the requested code contribute;
packs are merged in `order` asc (ties by id, later wins). Installing a
language pack therefore translates the entire dashboard chrome; deleting it
returns the UI to English (or to the next installed pack's locale).

- `initI18n()` runs before the app bundle loads (`src/main.tsx`): it fetches
  the persisted locale, then the catalog, so the first paint is already in
  the operator's language. The last successful catalog is cached in
  `localStorage`, so a signed-out boot (login page) still renders in the
  previously chosen language.
- Changing **Settings → Extensions → Interface language** saves the shared
  locale (same `PUT /sidecar/extensions/locale`) and reloads the page.
- RTL locales (`ar`, `he`) set `document.documentElement.dir = "rtl"`.
- After login/SSO the client re-syncs (`ensureLocaleSynced()`) and reloads if
  the persisted locale differs from the one the bundle started with.
- The language selector lists exactly what the gateway offers (installed
  packs + embedded extension i18n); English stays reachable via the
  "Source language" option.

Regenerating the pack `dashboard` catalogs after UI changes:

```bash
node scripts/i18n-extract-keys.cjs      # rebuild scripts/i18n-keys.json
# translators update scripts/i18n-translations/<code>.json (ordered arrays)
python3 scripts/i18n-merge-catalogs.py  # inject "dashboard" into the store packs
```

## See also

- [README.md](README.md) — extension layers, install/apply flow
- [THEMES.md](THEMES.md) — theme extensions (applied one at a time)
- [ADDONS.md](ADDONS.md) — Go addons contributing live UI
