// Mechanical i18n codemod for the Aurora dashboard.
//
// Wraps human-readable string literals in t() from "@/lib/i18n":
//   - JSX text nodes            <p>Save changes</p>      -> <p>{t("Save changes")}</p>
//   - whitelisted attributes    label="Save"              -> label={t("Save")}
//   - lone JSX string expr      {"Save"}                  -> {t("Save")}
//   - conditional strings       {busy ? "Saving…" : "Save"} -> {busy ? t("Saving…") : t("Save")}
//   - whitelisted object props  { label: "Rotate" }       -> { label: t("Rotate") }
//
// JSX text is normalized with Babel's cleanJSXElementLiteralChild algorithm so
// the fallback (t returns the key) renders byte-identical to the original.
// Strings without Latin letters, URLs, and all-lowercase single tokens
// (likely identifiers) are left untouched. Files under src/lib/i18n, tests and
// typings are skipped.
//
// Usage (inside the node:22 container with NODE_PATH=dashboard-ui/node_modules/.pnpm/node_modules):
//   node scripts/i18n-codemod.mjs [--dry-run] [files...]

"use strict";

const fs = require("fs");
const path = require("path");
const parser = require("@babel/parser");
const traverse = require("@babel/traverse").default;
const MagicString = require("magic-string");

const ROOT = path.resolve(__dirname, "..");
const SRC = path.join(ROOT, "src");

const IMPORT_LINE = 'import { t } from "@/lib/i18n";\n';
const FALLBACK_ALIAS = "i18nT";

// JSX attributes whose string values are human-readable.
const ATTR_WHITELIST = new Set([
  "label",
  "title",
  "description",
  "placeholder",
  "aria-label",
  "alt",
  "message",
  "tooltip",
  "help",
  "summary",
  "hint",
  "confirmLabel",
  "cancelLabel",
  "sublabel",
  "caption",
  "subtitle",
  "placeholderText",
  "emptyText",
]);

// Object-property keys whose string values are human-readable.
const PROP_WHITELIST = new Set([
  "label",
  "title",
  "description",
  "placeholder",
  "message",
  "tooltip",
  "help",
  "summary",
  "hint",
  "confirmLabel",
  "cancelLabel",
  "sublabel",
  "caption",
  "subtitle",
  "placeholderText",
  "emptyText",
  "text",
]);

// JSX elements whose text content must NOT be translated (code samples etc.).
const SKIP_ELEMENTS = new Set(["code", "pre", "kbd", "samp", "style", "script"]);

// File patterns to never touch.
const SKIP_FILES = [
  /\/lib\/i18n\//,
  /\.test\.tsx?$/,
  /\/tests\//,
  /env\.d\.ts$/,
  /routeTree\.gen\.ts$/,
];

const dryRun = process.argv.includes("--dry-run");
const explicitFiles = process.argv.slice(2).filter((a) => a !== "--dry-run");

function listFiles(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      listFiles(p, out);
    } else if (/\.tsx?$/.test(entry.name)) {
      out.push(p);
    }
  }
  return out;
}

// Replicates @babel/types cleanJSXElementLiteralChild: the string React
// actually renders for a JSXText node.
function normalizeJSXText(value) {
  const lines = value.split(/\r\n|\n|\r/);
  let lastNonEmptyLine = 0;
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].match(/[^ \t]/)) lastNonEmptyLine = i;
  }
  let str = "";
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const isFirstLine = i === 0;
    const isLastLine = i === lines.length - 1;
    const isLastNonEmptyLine = i === lastNonEmptyLine;
    let trimmedLine = line.replace(/\t/g, " ");
    if (!isFirstLine) trimmedLine = trimmedLine.replace(/^[ ]+/, "");
    if (!isLastLine) trimmedLine = trimmedLine.replace(/[ ]+$/, "");
    if (trimmedLine) {
      if (!isLastNonEmptyLine) trimmedLine += " ";
      str += trimmedLine;
    }
  }
  return str;
}

// Should this text be translated at all?
function translatable(text) {
  if (!/[A-Za-z]/.test(text)) return false; // no letters (punctuation, digits, —)
  if (/https?:\/\//.test(text)) return false;
  if (/\$\{/.test(text)) return false;
  // Single all-lowercase token without spaces is more likely an identifier
  // ("rotate", "fallback") than UI text.
  if (/^[a-z0-9._-]+$/.test(text)) return false;
  return true;
}

function parentElementNames(path) {
  const names = [];
  let p = path.parentPath;
  while (p) {
    if (p.isJSXElement() && p.node.openingElement) {
      const n = p.node.openingElement.name;
      if (n.type === "JSXIdentifier") names.push(n.name);
    }
    p = p.parentPath;
  }
  return names;
}

function processFile(file) {
  const rel = path.relative(ROOT, file);
  if (SKIP_FILES.some((re) => re.test(rel))) return null;

  const code = fs.readFileSync(file, "utf8");
  const isTsx = file.endsWith(".tsx");
  const plugins = isTsx ? ["typescript", "jsx"] : ["typescript"];
  let ast;
  try {
    ast = parser.parse(code, { sourceType: "module", plugins, errorRecovery: true });
  } catch (err) {
    console.error(`PARSE FAIL ${rel}: ${err.message}`);
    return null;
  }

  const ms = new MagicString(code);
  const edits = []; // {start, end, text}
  const keys = new Set();
  let usesT = 0;

  // Detect collisions: any pre-existing identifier named t (or our alias).
  let hasT = false;
  let hasAlias = false;
  traverse(ast, {
    Identifier(p) {
      if (p.node.name === "t") hasT = true;
      else if (p.node.name === FALLBACK_ALIAS) hasAlias = true;
    },
  });
  const callee = hasT ? (hasAlias ? null : FALLBACK_ALIAS) : "t";
  if (callee === null) {
    console.error(`SKIP ${rel}: both t and ${FALLBACK_ALIAS} already bound`);
    return null;
  }

  const quote = (s) => JSON.stringify(s);

  function addEdit(start, end, text) {
    edits.push({ start, end, text });
  }

  function recordKey(text) {
    const core = text.replace(/^\s+|\s+$/g, "");
    if (core) keys.add(core);
    usesT += 1;
  }

  traverse(ast, {
    // ---- JSX text nodes -------------------------------------------------
    JSXText(p) {
      const names = parentElementNames(p);
      if (names.some((n) => SKIP_ELEMENTS.has(n))) return;
      const normalized = normalizeJSXText(p.node.value);
      if (!translatable(normalized)) return;
      addEdit(p.node.start, p.node.end, `{${callee}(${quote(normalized)})}`);
      recordKey(normalized);
    },

    // ---- whitelisted JSX attributes ------------------------------------
    JSXAttribute(p) {
      const nameNode = p.node.name;
      const attrName = nameNode.type === "JSXIdentifier" ? nameNode.name : null;
      if (!attrName || !ATTR_WHITELIST.has(attrName)) return;
      const v = p.node.value;
      if (!v || v.type !== "StringLiteral") return;
      if (!translatable(v.value)) return;
      addEdit(v.start, v.end, `{${callee}(${quote(v.value)})}`);
      recordKey(v.value);
    },

    // ---- string literals inside JSX expressions ------------------------
    StringLiteral(p) {
      // Only handle when reached from a JSXExpressionContainer through
      // conditional/logical positions; object properties handled below.
      if (p.parentPath.isJSXExpressionContainer()) {
        if (!translatable(p.node.value)) return;
        addEdit(p.node.start, p.node.end, `${callee}(${quote(p.node.value)})`);
        recordKey(p.node.value);
        return;
      }
      let child = p;
      let parent = p.parentPath;
      let ok = false;
      while (parent) {
        if (parent.isJSXExpressionContainer()) {
          ok = true;
          break;
        }
        if (parent.isConditionalExpression()) {
          // Do not translate the test position.
          if (child.node !== parent.node.test) {
            child = parent;
            parent = parent.parentPath;
            continue;
          }
          return;
        }
        if (parent.isLogicalExpression()) {
          // Only the right operand ("x || None") is display text.
          if (child.node === parent.node.right) {
            child = parent;
            parent = parent.parentPath;
            continue;
          }
          return;
        }
        return;
      }
      if (!ok) return;
      if (!translatable(p.node.value)) return;
      addEdit(p.node.start, p.node.end, `${callee}(${quote(p.node.value)})`);
      recordKey(p.node.value);
    },

    // ---- whitelisted object properties ---------------------------------
    ObjectProperty(p) {
      const keyNode = p.node.key;
      let keyName = null;
      if (keyNode.type === "Identifier") keyName = keyNode.name;
      else if (keyNode.type === "StringLiteral") keyName = keyNode.value;
      if (!keyName || !PROP_WHITELIST.has(keyName)) return;
      if (p.node.computed) return;
      const v = p.node.value;
      if (v.type !== "StringLiteral") return;
      if (!translatable(v.value)) return;
      addEdit(v.start, v.end, `${callee}(${quote(v.value)})`);
      recordKey(v.value);
    },
  });

  if (!usesT) return null;

  // Insert the import after the last top-level import (or at the top).
  const body = ast.program.body;
  let insertAt = 0;
  for (const node of body) {
    if (node.type === "ImportDeclaration") insertAt = Math.max(insertAt, node.end);
  }
  const importText =
    callee === "t"
      ? IMPORT_LINE
      : `import { t as ${FALLBACK_ALIAS} } from "@/lib/i18n";\n`;

  // Apply edits back-to-front so offsets stay valid.
  edits.sort((a, b) => b.start - a.start);
  for (const e of edits) ms.overwrite(e.start, e.end, e.text);
  ms.appendLeft(insertAt, insertAt === 0 ? importText : "\n" + importText);

  const result = ms.toString();
  if (!dryRun) fs.writeFileSync(file, result);
  return { rel, count: usesT, keys: [...keys] };
}

const files = explicitFiles.length ? explicitFiles.map((f) => path.resolve(f)) : listFiles(SRC);
const report = [];
let total = 0;
let totalKeys = new Set();
for (const f of files) {
  const r = processFile(f);
  if (r) {
    report.push(r);
    total += r.count;
    r.keys.forEach((k) => totalKeys.add(k));
    console.log(`${dryRun ? "DRY " : ""}${r.rel}: ${r.count} strings`);
  }
}
console.log(`\n${report.length} files, ${total} strings, ${totalKeys.size} unique keys`);
fs.writeFileSync(
  path.join(ROOT, "scripts", "i18n-keys.json"),
  JSON.stringify([...totalKeys].sort(), null, 2) + "\n",
);
console.log(`keys written to scripts/i18n-keys.json`);
