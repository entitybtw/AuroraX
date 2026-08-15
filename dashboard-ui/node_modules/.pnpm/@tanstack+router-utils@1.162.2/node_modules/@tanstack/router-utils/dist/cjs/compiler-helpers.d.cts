import * as t from '@babel/types';
type CompilerNodePath<TNode extends t.Node = t.Node> = {
    node: TNode;
    parentPath: CompilerNodePath | null;
    isVariableDeclarator: () => boolean;
};
type ReplacePathNode<TPath, TNode extends t.Node> = Omit<TPath, 'node'> & {
    node: TNode;
};
export type ModuleInfoBinding = {
    type: 'import';
    source: string;
    importedName: string;
} | {
    type: 'var';
    init: t.Expression | null;
};
export interface ExtractedModuleInfo {
    bindings: Map<string, ModuleInfoBinding>;
    exports: Map<string, string>;
    reExportAllSources: Array<string>;
}
export declare function unwrapExpression(expr: t.Expression): t.Expression;
export declare function getVariableDeclaratorForExpressionPath<TPath extends CompilerNodePath<t.Expression>>(path: TPath): ReplacePathNode<TPath, t.VariableDeclarator> | null;
/**
 * Recursively walk an AST node and collect referenced identifier-like names.
 * This avoids Babel path/scope allocation for module-level dependency scans.
 */
export declare function collectIdentifiersFromNode(node: t.Node): Set<string>;
export declare function collectIdentifiersFromPattern(node: t.LVal | t.Node | null | undefined): Array<string>;
export declare function collectLocalBindingsFromStatement(node: t.Statement | t.ModuleDeclaration, bindings: Set<string>): void;
export declare function extractModuleInfoFromAst(ast: t.File): ExtractedModuleInfo;
export declare function buildDeclarationMap(ast: t.File): Map<string, t.Node>;
export declare function buildDependencyGraph(declarationMap: Map<string, t.Node>, localBindings: Set<string>): Map<string, Set<string>>;
export declare function collectModuleLevelRefsFromNode(node: t.Node, localModuleLevelBindings: Set<string>): Set<string>;
export declare function expandTransitively(bindings: Set<string>, dependencyGraph: Map<string, Set<string>>): void;
export declare function expandSharedDestructuredDeclarators(ast: t.File, refsByGroup: Map<string, Set<number>>, sharedBindings: Set<string>): void;
export declare function expandDestructuredDeclarations(ast: t.File, bindings: Set<string>): void;
export declare function removeBindingsTransitivelyDependingOn(bindings: Set<string>, dependencyGraph: Map<string, Set<string>>, roots: Iterable<string>): void;
export declare function removeModuleLevelBindings(ast: t.File, namesToRemove: Set<string>): void;
export declare function retainModuleLevelDeclarations(ast: t.File, bindingsToKeep: Set<string>): void;
export declare function unwrapExportedDeclarations(ast: t.File): void;
export declare function stripUnreferencedTopLevelExpressionStatements(ast: t.File): void;
export {};
