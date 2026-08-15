const require_runtime = require("./_virtual/_rolldown/runtime.cjs");
let _babel_parser = require("@babel/parser");
let _babel_generator = require("@babel/generator");
_babel_generator = require_runtime.__toESM(_babel_generator, 1);
let _babel_types = require("@babel/types");
_babel_types = require_runtime.__toESM(_babel_types, 1);
let babel_dead_code_elimination = require("babel-dead-code-elimination");
//#region src/ast.ts
function parseAst({ code, filename, sourceFilename, plugins, ...opts }) {
	return (0, _babel_parser.parse)(code, {
		plugins: plugins ?? getDefaultParserPluginsForFilename(filename ?? sourceFilename),
		sourceType: "module",
		sourceFilename,
		...opts
	});
}
function getDefaultParserPluginsForFilename(filename) {
	const plugins = [
		"typescript",
		"explicitResourceManagement",
		"importAttributes",
		"deprecatedImportAssert",
		["decorators", { decoratorsBeforeExport: true }],
		"decoratorAutoAccessors"
	];
	if (!isPlainTypeScriptFile(filename)) plugins.unshift("jsx");
	return plugins;
}
function isPlainTypeScriptFile(filename) {
	if (!filename) return false;
	return /\.[cm]?ts(?:$|[?#])/.test(filename);
}
var generate = _babel_generator.default;
if ("default" in generate) generate = generate.default;
function generateFromAst(ast, opts) {
	return generate(ast, opts ? {
		importAttributesKeyword: "with",
		sourceMaps: true,
		...opts
	} : void 0);
}
/**
* Strips TypeScript type-only exports and imports from an AST.
*
* This is necessary because babel-dead-code-elimination doesn't handle
* TypeScript type exports/imports. When a type export references an import
* that pulls in server-only code, the dead code elimination won't remove
* that import because it sees the type as still referencing it.
*
* This function removes:
* - `export type Foo = ...`
* - `export interface Foo { ... }`
* - `export type { Foo } from './module'`
* - `export type * from './module'`
* - Type specifiers in mixed exports: `export { value, type Foo }` -> `export { value }`
* - `import type { Foo } from './module'`
* - Type specifiers in mixed imports: `import { value, type Foo } from './module'` -> `import { value }`
*
* Note: Non-exported type/interface declarations are preserved as they may be
* used as type annotations within the code.
*
* @param ast - The Babel AST (or ParseResult) to mutate
*/
function stripTypeExports(ast) {
	ast.program.body = ast.program.body.filter((node) => {
		if (_babel_types.isExportNamedDeclaration(node)) {
			if (node.exportKind === "type") return false;
			if (node.specifiers.length > 0) {
				node.specifiers = node.specifiers.filter((specifier) => {
					if (_babel_types.isExportSpecifier(specifier)) return specifier.exportKind !== "type";
					return true;
				});
				if (node.specifiers.length === 0 && !node.declaration) return false;
			}
		}
		if (_babel_types.isExportAllDeclaration(node)) {
			if (node.exportKind === "type") return false;
		}
		if (_babel_types.isImportDeclaration(node)) {
			if (node.importKind === "type") return false;
			if (node.specifiers.length > 0) {
				node.specifiers = node.specifiers.filter((specifier) => {
					if (_babel_types.isImportSpecifier(specifier)) return specifier.importKind !== "type";
					return true;
				});
				if (node.specifiers.length === 0) return false;
			}
		}
		return true;
	});
}
/**
* Performs dead code elimination on the AST, with TypeScript type stripping.
*
* This is a wrapper around babel-dead-code-elimination that first strips
* TypeScript type-only exports and imports. This is necessary because
* babel-dead-code-elimination doesn't handle type exports, which can cause
* imports to be retained when they're only referenced by type exports.
*
* @param ast - The Babel AST to mutate
* @param candidates - Optional set of identifier paths to consider for removal.
*                     If provided, only these identifiers will be candidates for removal.
*                     This should be the result of `findReferencedIdentifiers(ast)` called
*                     before any AST transformations.
*/
function deadCodeElimination(ast, candidates) {
	stripTypeExports(ast);
	(0, babel_dead_code_elimination.deadCodeElimination)(ast, candidates);
}
//#endregion
exports.deadCodeElimination = deadCodeElimination;
exports.generateFromAst = generateFromAst;
exports.parseAst = parseAst;
exports.stripTypeExports = stripTypeExports;

//# sourceMappingURL=ast.cjs.map