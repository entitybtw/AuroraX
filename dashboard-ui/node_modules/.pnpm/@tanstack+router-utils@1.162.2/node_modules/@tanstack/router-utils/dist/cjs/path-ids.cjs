//#region src/path-ids.ts
function createIdentifier(strings) {
	if (strings.length === 0) throw new Error("Cannot create an identifier from an empty array");
	let safeString = [...strings].sort().join("---").replace(/\//g, "--slash--");
	safeString = safeString.replace(/\\/g, "--backslash--");
	safeString = safeString.replace(/\?/g, "--question--");
	safeString = safeString.replace(/%/g, "--percent--");
	safeString = safeString.replace(/#/g, "--hash--");
	safeString = safeString.replace(/\+/g, "--plus--");
	safeString = safeString.replace(/=/g, "--equals--");
	safeString = safeString.replace(/&/g, "--ampersand--");
	safeString = safeString.replace(/\s/g, "_");
	return safeString;
}
function decodeIdentifier(identifier) {
	if (!identifier) return [];
	let combinedString = identifier.replace(/--slash--/g, "/");
	combinedString = combinedString.replace(/--backslash--/g, "\\");
	combinedString = combinedString.replace(/--question--/g, "?");
	combinedString = combinedString.replace(/--percent--/g, "%");
	combinedString = combinedString.replace(/--hash--/g, "#");
	combinedString = combinedString.replace(/--plus--/g, "+");
	combinedString = combinedString.replace(/--equals--/g, "=");
	combinedString = combinedString.replace(/--ampersand--/g, "&");
	combinedString = combinedString.replace(/_/g, " ");
	return combinedString.split("---");
}
//#endregion
exports.createIdentifier = createIdentifier;
exports.decodeIdentifier = decodeIdentifier;

//# sourceMappingURL=path-ids.cjs.map