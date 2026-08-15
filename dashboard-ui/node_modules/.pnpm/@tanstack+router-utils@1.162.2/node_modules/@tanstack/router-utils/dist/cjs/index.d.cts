export { parseAst, generateFromAst, deadCodeElimination, findReferencedIdentifiers, stripTypeExports, } from './ast.cjs';
export type { ParseAstOptions, ParseAstResult, GeneratorResult } from './ast.cjs';
export { logDiff } from './logger.cjs';
export { copyFilesPlugin } from './copy-files-plugin.cjs';
export { createIdentifier, decodeIdentifier } from './path-ids.cjs';
export { buildDeclarationMap, buildDependencyGraph, collectIdentifiersFromNode, collectIdentifiersFromPattern, collectLocalBindingsFromStatement, collectModuleLevelRefsFromNode, expandDestructuredDeclarations, expandSharedDestructuredDeclarators, expandTransitively, extractModuleInfoFromAst, getVariableDeclaratorForExpressionPath, removeBindingsTransitivelyDependingOn, removeModuleLevelBindings, retainModuleLevelDeclarations, stripUnreferencedTopLevelExpressionStatements, unwrapExpression, unwrapExportedDeclarations, } from './compiler-helpers.cjs';
export type { ExtractedModuleInfo, ModuleInfoBinding } from './compiler-helpers.cjs';
