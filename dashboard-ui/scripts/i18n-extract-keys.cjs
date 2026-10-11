// Extract every t("...") / t('...') key from src into scripts/i18n-keys.json.
// Run with NODE_PATH=dashboard-ui/node_modules/.pnpm/node_modules.

"use strict";

const fs = require("fs");
const path = require("path");
const parser = require("@babel/parser");
const traverse = require("@babel/traverse").default;

const SRC = path.resolve(__dirname, "..", "src");
const keys = new Set();

function walk(dir) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (entry.name === "i18n") continue; // skip the runtime itself
      walk(p);
    } else if (/\.tsx?$/.test(entry.name) && !/\.test\.tsx?$/.test(entry.name)) {
      const code = fs.readFileSync(p, "utf8");
      let ast;
      try {
        ast = parser.parse(code, {
          sourceType: "module",
          plugins: entry.name.endsWith(".tsx") ? ["typescript", "jsx"] : ["typescript"],
        });
      } catch (err) {
        console.error(`PARSE FAIL ${p}: ${err.message}`);
        continue;
      }
      traverse(ast, {
        CallExpression(cp) {
          const callee = cp.node.callee;
          if (
            callee.type === "Identifier" &&
            (callee.name === "t" || callee.name === "i18nT") &&
            cp.node.arguments.length >= 1 &&
            cp.node.arguments[0].type === "StringLiteral"
          ) {
            // t() trims the key for the catalog lookup, so store the trimmed
            // canonical form (leading/trailing spaces are re-applied by t()).
            const raw = cp.node.arguments[0].value;
            const core = raw.replace(/^\s+|\s+$/g, "");
            if (core) keys.add(core);
          }
        },
      });
    }
  }
}

walk(SRC);
const sorted = [...keys].sort();
fs.writeFileSync(
  path.resolve(__dirname, "i18n-keys.json"),
  JSON.stringify(sorted, null, 2) + "\n",
);
console.log(`${sorted.length} keys`);
