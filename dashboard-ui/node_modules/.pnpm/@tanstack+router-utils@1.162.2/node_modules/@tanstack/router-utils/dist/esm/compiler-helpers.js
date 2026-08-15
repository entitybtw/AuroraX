import * as t from "@babel/types";
//#region src/compiler-helpers.ts
function getTransparentWrapperExpression(node) {
	if (t.isTSAsExpression(node) || t.isTSSatisfiesExpression(node) || t.isTSTypeAssertion(node) || t.isTSNonNullExpression(node) || t.isParenthesizedExpression(node)) return node.expression;
	return null;
}
function unwrapExpression(expr) {
	let inner = getTransparentWrapperExpression(expr);
	while (inner) {
		expr = inner;
		inner = getTransparentWrapperExpression(expr);
	}
	return expr;
}
function getVariableDeclaratorForExpressionPath(path) {
	let currentPath = path;
	let parentPath = currentPath.parentPath;
	while (parentPath && getTransparentWrapperExpression(parentPath.node) === currentPath.node) {
		currentPath = parentPath;
		parentPath = parentPath.parentPath;
	}
	if (parentPath?.isVariableDeclarator() && t.isVariableDeclarator(parentPath.node) && parentPath.node.init === currentPath.node) return parentPath;
	return null;
}
function getModuleExportName(node) {
	return t.isIdentifier(node) ? node.name : node.value;
}
function addVariableDeclarationModuleInfo(declaration, bindings, exportMap) {
	for (const declarator of declaration.declarations) for (const name of collectIdentifiersFromPattern(declarator.id)) {
		bindings.set(name, {
			type: "var",
			init: declarator.init ?? null
		});
		exportMap?.set(name, name);
	}
}
function addDeclarationModuleInfo(declaration, bindings, exportMap) {
	if (t.isVariableDeclaration(declaration)) {
		addVariableDeclarationModuleInfo(declaration, bindings, exportMap);
		return;
	}
	if ((t.isFunctionDeclaration(declaration) || t.isClassDeclaration(declaration)) && declaration.id) {
		bindings.set(declaration.id.name, {
			type: "var",
			init: null
		});
		exportMap?.set(declaration.id.name, declaration.id.name);
	}
}
function hasIdentifierBinding(scopes, name) {
	for (let i = scopes.length - 1; i >= 0; i--) if (scopes[i].bindings.has(name)) return true;
	return false;
}
function currentIdentifierScope(scopes) {
	return scopes[scopes.length - 1];
}
function nearestFunctionIdentifierScope(scopes) {
	for (let i = scopes.length - 1; i >= 0; i--) {
		const scope = scopes[i];
		if (scope.kind === "function" || scope.kind === "program") return scope;
	}
	return currentIdentifierScope(scopes);
}
function addIdentifierPatternBindings(pattern, scope) {
	for (const name of collectIdentifiersFromPattern(pattern)) scope.bindings.add(name);
}
function addIdentifierDeclarationBindings(declaration, scopes) {
	if (t.isVariableDeclaration(declaration)) {
		const scope = declaration.kind === "var" ? nearestFunctionIdentifierScope(scopes) : currentIdentifierScope(scopes);
		for (const declarator of declaration.declarations) addIdentifierPatternBindings(declarator.id, scope);
		return;
	}
	if ((t.isFunctionDeclaration(declaration) || t.isClassDeclaration(declaration) || t.isTSTypeAliasDeclaration(declaration) || t.isTSInterfaceDeclaration(declaration) || t.isTSEnumDeclaration(declaration)) && declaration.id) currentIdentifierScope(scopes).bindings.add(declaration.id.name);
}
function addIdentifierImportBindings(node, scope) {
	for (const specifier of node.specifiers) scope.bindings.add(specifier.local.name);
}
function createNestedIdentifierScope(kind, scopes) {
	return [...scopes, {
		kind,
		bindings: /* @__PURE__ */ new Set()
	}];
}
function addIdentifierBlockBindings(body, scopes) {
	for (const statement of body) if (t.isImportDeclaration(statement)) addIdentifierImportBindings(statement, currentIdentifierScope(scopes));
	else if (t.isExportNamedDeclaration(statement) && statement.declaration) addIdentifierDeclarationBindings(statement.declaration, scopes);
	else addIdentifierDeclarationBindings(statement, scopes);
}
function walkIdentifierChildren(current, parent, scopes, ids) {
	for (const key of t.VISITOR_KEYS[current.type] ?? []) {
		const child = current[key];
		if (Array.isArray(child)) {
			for (const item of child) if (item && typeof item.type === "string") walkIdentifierNode(item, current, parent, key, scopes, ids);
		} else if (child && typeof child.type === "string") walkIdentifierNode(child, current, parent, key, scopes, ids);
	}
}
function walkIdentifierNode(current, parent, grandparent, parentKey, scopes, ids) {
	if (!current) return;
	if (t.isIdentifier(current)) {
		if ((!parent || t.isReferenced(current, parent, grandparent)) && !hasIdentifierBinding(scopes, current.name)) ids.add(current.name);
		return;
	}
	if (t.isJSXIdentifier(current)) {
		if (parent && t.isJSXAttribute(parent) && parentKey === "name") return;
		if (parent && t.isJSXMemberExpression(parent) && parentKey === "property") return;
		const first = current.name[0];
		if (first && first === first.toLowerCase()) return;
		if (!hasIdentifierBinding(scopes, current.name)) ids.add(current.name);
		return;
	}
	if (t.isProgram(current)) {
		const nestedScopes = createNestedIdentifierScope("program", scopes);
		addIdentifierBlockBindings(current.body, nestedScopes);
		for (const child of current.body) walkIdentifierNode(child, current, parent, "body", nestedScopes, ids);
		return;
	}
	if (t.isBlockStatement(current)) {
		const nestedScopes = createNestedIdentifierScope("block", scopes);
		addIdentifierBlockBindings(current.body, nestedScopes);
		for (const child of current.body) walkIdentifierNode(child, current, parent, "body", nestedScopes, ids);
		return;
	}
	if (t.isFunctionDeclaration(current) || t.isFunctionExpression(current) || t.isArrowFunctionExpression(current) || t.isObjectMethod(current) || t.isClassMethod(current) || t.isClassPrivateMethod(current)) {
		if (t.isFunctionDeclaration(current) && current.id) currentIdentifierScope(scopes).bindings.add(current.id.name);
		const nestedScopes = createNestedIdentifierScope("function", scopes);
		if ((t.isFunctionDeclaration(current) || t.isFunctionExpression(current)) && current.id) currentIdentifierScope(nestedScopes).bindings.add(current.id.name);
		for (const param of current.params) addIdentifierPatternBindings(param, currentIdentifierScope(nestedScopes));
		walkIdentifierChildren(current, parent, nestedScopes, ids);
		return;
	}
	if (t.isCatchClause(current)) {
		const nestedScopes = createNestedIdentifierScope("block", scopes);
		addIdentifierPatternBindings(current.param, currentIdentifierScope(nestedScopes));
		walkIdentifierNode(current.param, current, parent, "param", nestedScopes, ids);
		walkIdentifierNode(current.body, current, parent, "body", nestedScopes, ids);
		return;
	}
	if (t.isImportDeclaration(current)) {
		addIdentifierImportBindings(current, currentIdentifierScope(scopes));
		return;
	}
	if (t.isClassDeclaration(current) || t.isClassExpression(current)) {
		if (t.isClassDeclaration(current) && current.id) currentIdentifierScope(scopes).bindings.add(current.id.name);
		const nestedScopes = current.id ? createNestedIdentifierScope("block", scopes) : scopes;
		if (current.id) currentIdentifierScope(nestedScopes).bindings.add(current.id.name);
		walkIdentifierChildren(current, parent, nestedScopes, ids);
		return;
	}
	if (t.isVariableDeclaration(current)) addIdentifierDeclarationBindings(current, scopes);
	else if (t.isVariableDeclarator(current)) {
		const scope = parent && t.isVariableDeclaration(parent) && parent.kind === "var" ? nearestFunctionIdentifierScope(scopes) : currentIdentifierScope(scopes);
		addIdentifierPatternBindings(current.id, scope);
	} else if (t.isTSTypeAliasDeclaration(current) || t.isTSInterfaceDeclaration(current) || t.isTSEnumDeclaration(current)) currentIdentifierScope(scopes).bindings.add(current.id.name);
	walkIdentifierChildren(current, parent, scopes, ids);
}
/**
* Recursively walk an AST node and collect referenced identifier-like names.
* This avoids Babel path/scope allocation for module-level dependency scans.
*/
function collectIdentifiersFromNode(node) {
	const ids = /* @__PURE__ */ new Set();
	walkIdentifierNode(node, void 0, void 0, void 0, [{
		kind: "program",
		bindings: /* @__PURE__ */ new Set()
	}], ids);
	return ids;
}
function collectIdentifiersFromPattern(node) {
	if (!node) return [];
	if (t.isIdentifier(node)) return [node.name];
	if (t.isAssignmentPattern(node)) return collectIdentifiersFromPattern(node.left);
	if (t.isRestElement(node)) return collectIdentifiersFromPattern(node.argument);
	if (t.isObjectPattern(node)) return node.properties.flatMap((prop) => {
		if (t.isObjectProperty(prop)) return collectIdentifiersFromPattern(prop.value);
		if (t.isRestElement(prop)) return collectIdentifiersFromPattern(prop.argument);
		return [];
	});
	if (t.isArrayPattern(node)) return node.elements.flatMap((element) => collectIdentifiersFromPattern(element));
	return [];
}
function collectLocalBindingsFromStatement(node, bindings) {
	const declaration = t.isExportNamedDeclaration(node) && node.declaration ? node.declaration : t.isExportDefaultDeclaration(node) ? node.declaration : node;
	if (t.isVariableDeclaration(declaration)) for (const declarator of declaration.declarations) for (const name of collectIdentifiersFromPattern(declarator.id)) bindings.add(name);
	else if (t.isFunctionDeclaration(declaration) && declaration.id) bindings.add(declaration.id.name);
	else if (t.isClassDeclaration(declaration) && declaration.id) bindings.add(declaration.id.name);
}
function extractModuleInfoFromAst(ast) {
	const bindings = /* @__PURE__ */ new Map();
	const exportMap = /* @__PURE__ */ new Map();
	const reExportAllSources = [];
	for (const node of ast.program.body) {
		if (t.isImportDeclaration(node)) {
			const source = node.source.value;
			for (const specifier of node.specifiers) if (t.isImportSpecifier(specifier)) bindings.set(specifier.local.name, {
				type: "import",
				source,
				importedName: getModuleExportName(specifier.imported)
			});
			else if (t.isImportDefaultSpecifier(specifier)) bindings.set(specifier.local.name, {
				type: "import",
				source,
				importedName: "default"
			});
			else if (t.isImportNamespaceSpecifier(specifier)) bindings.set(specifier.local.name, {
				type: "import",
				source,
				importedName: "*"
			});
			continue;
		}
		if (t.isVariableDeclaration(node)) {
			addVariableDeclarationModuleInfo(node, bindings);
			continue;
		}
		if (t.isFunctionDeclaration(node) || t.isClassDeclaration(node)) {
			addDeclarationModuleInfo(node, bindings);
			continue;
		}
		if (t.isExportNamedDeclaration(node)) {
			if (node.declaration) addDeclarationModuleInfo(node.declaration, bindings, exportMap);
			for (const specifier of node.specifiers) if (t.isExportNamespaceSpecifier(specifier)) {
				const exported = getModuleExportName(specifier.exported);
				exportMap.set(exported, exported);
				if (node.source) bindings.set(exported, {
					type: "import",
					source: node.source.value,
					importedName: "*"
				});
			} else if (t.isExportSpecifier(specifier)) {
				const local = getModuleExportName(specifier.local);
				const exported = getModuleExportName(specifier.exported);
				exportMap.set(exported, local);
				if (node.source) bindings.set(local, {
					type: "import",
					source: node.source.value,
					importedName: local
				});
			}
			continue;
		}
		if (t.isExportDefaultDeclaration(node)) {
			const declaration = node.declaration;
			if (t.isIdentifier(declaration)) exportMap.set("default", declaration.name);
			else if ((t.isFunctionDeclaration(declaration) || t.isClassDeclaration(declaration)) && declaration.id) {
				bindings.set(declaration.id.name, {
					type: "var",
					init: null
				});
				exportMap.set("default", declaration.id.name);
			} else {
				const synth = "__default_export__";
				bindings.set(synth, {
					type: "var",
					init: t.isExpression(declaration) ? declaration : null
				});
				exportMap.set("default", synth);
			}
			continue;
		}
		if (t.isExportAllDeclaration(node)) reExportAllSources.push(node.source.value);
	}
	return {
		bindings,
		exports: exportMap,
		reExportAllSources
	};
}
function buildDeclarationMap(ast) {
	const map = /* @__PURE__ */ new Map();
	for (const statement of ast.program.body) {
		const declaration = t.isExportNamedDeclaration(statement) && statement.declaration ? statement.declaration : t.isExportDefaultDeclaration(statement) ? statement.declaration : statement;
		if (t.isVariableDeclaration(declaration)) for (const declarator of declaration.declarations) for (const name of collectIdentifiersFromPattern(declarator.id)) map.set(name, declarator);
		else if (t.isFunctionDeclaration(declaration) && declaration.id) map.set(declaration.id.name, declaration);
		else if (t.isClassDeclaration(declaration) && declaration.id) map.set(declaration.id.name, declaration);
	}
	return map;
}
function buildDependencyGraph(declarationMap, localBindings) {
	const graph = /* @__PURE__ */ new Map();
	for (const [name, declarationNode] of declarationMap) {
		if (!localBindings.has(name)) continue;
		const dependencies = /* @__PURE__ */ new Set();
		for (const id of collectIdentifiersFromNode(declarationNode)) if (id !== name && localBindings.has(id)) dependencies.add(id);
		graph.set(name, dependencies);
	}
	return graph;
}
function collectModuleLevelRefsFromNode(node, localModuleLevelBindings) {
	const refs = /* @__PURE__ */ new Set();
	for (const name of collectIdentifiersFromNode(node)) if (localModuleLevelBindings.has(name)) refs.add(name);
	return refs;
}
function expandTransitively(bindings, dependencyGraph) {
	const queue = [...bindings];
	const visited = /* @__PURE__ */ new Set();
	while (queue.length > 0) {
		const name = queue.pop();
		if (visited.has(name)) continue;
		visited.add(name);
		const dependencies = dependencyGraph.get(name);
		if (!dependencies) continue;
		for (const dependency of dependencies) if (!bindings.has(dependency)) {
			bindings.add(dependency);
			queue.push(dependency);
		}
	}
}
function expandSharedDestructuredDeclarators(ast, refsByGroup, sharedBindings) {
	for (const statement of ast.program.body) {
		const declaration = t.isExportNamedDeclaration(statement) && statement.declaration ? statement.declaration : statement;
		if (!t.isVariableDeclaration(declaration)) continue;
		for (const declarator of declaration.declarations) {
			if (!t.isObjectPattern(declarator.id) && !t.isArrayPattern(declarator.id)) continue;
			const names = collectIdentifiersFromPattern(declarator.id);
			const usedGroups = /* @__PURE__ */ new Set();
			for (const name of names) {
				const groups = refsByGroup.get(name);
				if (!groups) continue;
				for (const group of groups) usedGroups.add(group);
			}
			if (usedGroups.size >= 2) for (const name of names) sharedBindings.add(name);
		}
	}
}
function expandDestructuredDeclarations(ast, bindings) {
	for (const statement of ast.program.body) {
		const declaration = t.isExportNamedDeclaration(statement) && statement.declaration ? statement.declaration : statement;
		if (!t.isVariableDeclaration(declaration)) continue;
		for (const declarator of declaration.declarations) {
			if (!t.isObjectPattern(declarator.id) && !t.isArrayPattern(declarator.id)) continue;
			const names = collectIdentifiersFromPattern(declarator.id);
			if (names.some((name) => bindings.has(name))) for (const name of names) bindings.add(name);
		}
	}
}
function removeBindingsTransitivelyDependingOn(bindings, dependencyGraph, roots) {
	const reverseGraph = /* @__PURE__ */ new Map();
	for (const [name, dependencies] of dependencyGraph) for (const dependency of dependencies) {
		let parents = reverseGraph.get(dependency);
		if (!parents) {
			parents = /* @__PURE__ */ new Set();
			reverseGraph.set(dependency, parents);
		}
		parents.add(name);
	}
	const visited = /* @__PURE__ */ new Set();
	const queue = [...roots];
	while (queue.length > 0) {
		const current = queue.pop();
		if (visited.has(current)) continue;
		visited.add(current);
		const parents = reverseGraph.get(current);
		if (!parents) continue;
		for (const parent of parents) if (!visited.has(parent)) queue.push(parent);
	}
	for (const name of [...bindings]) if (visited.has(name)) bindings.delete(name);
}
function removeModuleLevelBindings(ast, namesToRemove) {
	ast.program.body = ast.program.body.filter((statement) => {
		const declaration = t.isExportNamedDeclaration(statement) && statement.declaration ? statement.declaration : statement;
		if (t.isVariableDeclaration(declaration)) {
			declaration.declarations = declaration.declarations.filter((declarator) => !collectIdentifiersFromPattern(declarator.id).some((name) => namesToRemove.has(name)));
			return declaration.declarations.length > 0;
		}
		if (t.isFunctionDeclaration(declaration) && declaration.id) return !namesToRemove.has(declaration.id.name);
		if (t.isClassDeclaration(declaration) && declaration.id) return !namesToRemove.has(declaration.id.name);
		if (t.isExportDefaultDeclaration(statement)) {
			const defaultDeclaration = statement.declaration;
			if ((t.isFunctionDeclaration(defaultDeclaration) || t.isClassDeclaration(defaultDeclaration)) && defaultDeclaration.id) return !namesToRemove.has(defaultDeclaration.id.name);
		}
		return true;
	});
}
function retainModuleLevelDeclarations(ast, bindingsToKeep) {
	ast.program.body = ast.program.body.filter((statement) => {
		if (t.isImportDeclaration(statement)) return true;
		const declaration = t.isExportNamedDeclaration(statement) && statement.declaration ? statement.declaration : statement;
		if (t.isVariableDeclaration(declaration)) {
			declaration.declarations = declaration.declarations.filter((declarator) => collectIdentifiersFromPattern(declarator.id).some((name) => bindingsToKeep.has(name)));
			return declaration.declarations.length > 0;
		}
		if (t.isFunctionDeclaration(declaration) && declaration.id) return bindingsToKeep.has(declaration.id.name);
		if (t.isClassDeclaration(declaration) && declaration.id) return bindingsToKeep.has(declaration.id.name);
		return false;
	});
}
function unwrapExportedDeclarations(ast) {
	const body = [];
	for (const statement of ast.program.body) {
		if (t.isExportNamedDeclaration(statement)) {
			if (statement.declaration) body.push(statement.declaration);
			continue;
		}
		if (t.isExportDefaultDeclaration(statement)) {
			const declaration = statement.declaration;
			if ((t.isFunctionDeclaration(declaration) || t.isClassDeclaration(declaration)) && declaration.id) body.push(declaration);
			continue;
		}
		if (t.isExportAllDeclaration(statement)) continue;
		body.push(statement);
	}
	ast.program.body = body;
}
function stripUnreferencedTopLevelExpressionStatements(ast) {
	const locallyBound = /* @__PURE__ */ new Set();
	for (const statement of ast.program.body) collectLocalBindingsFromStatement(statement, locallyBound);
	ast.program.body = ast.program.body.filter((statement) => {
		if (!t.isExpressionStatement(statement)) return true;
		for (const name of collectIdentifiersFromNode(statement)) if (locallyBound.has(name)) return true;
		return false;
	});
}
//#endregion
export { buildDeclarationMap, buildDependencyGraph, collectIdentifiersFromNode, collectIdentifiersFromPattern, collectLocalBindingsFromStatement, collectModuleLevelRefsFromNode, expandDestructuredDeclarations, expandSharedDestructuredDeclarators, expandTransitively, extractModuleInfoFromAst, getVariableDeclaratorForExpressionPath, removeBindingsTransitivelyDependingOn, removeModuleLevelBindings, retainModuleLevelDeclarations, stripUnreferencedTopLevelExpressionStatements, unwrapExportedDeclarations, unwrapExpression };

//# sourceMappingURL=compiler-helpers.js.map