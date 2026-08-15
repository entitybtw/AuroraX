export { parseAst, generateFromAst, deadCodeElimination, findReferencedIdentifiers, stripTypeExports, } from './ast.js';
export type { ParseAstOptions, ParseAstResult, GeneratorResult } from './ast.js';
export { logDiff } from './logger.js';
export { copyFilesPlugin } from './copy-files-plugin.js';
export { createIdentifier, decodeIdentifier } from './path-ids.js';
export { buildDeclarationMap, buildDependencyGraph, collectIdentifiersFromNode, collectIdentifiersFromPattern, collectLocalBindingsFromStatement, collectModuleLevelRefsFromNode, expandDestructuredDeclarations, expandSharedDestructuredDeclarators, expandTransitively, extractModuleInfoFromAst, getVariableDeclaratorForExpressionPath, removeBindingsTransitivelyDependingOn, removeModuleLevelBindings, retainModuleLevelDeclarations, stripUnreferencedTopLevelExpressionStatements, unwrapExpression, unwrapExportedDeclarations, } from './compiler-helpers.js';
export type { ExtractedModuleInfo, ModuleInfoBinding } from './compiler-helpers.js';
