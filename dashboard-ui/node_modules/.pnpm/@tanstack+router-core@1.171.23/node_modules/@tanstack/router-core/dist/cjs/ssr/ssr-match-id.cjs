//#region src/ssr/ssr-match-id.ts
function dehydrateSsrMatchId(id) {
	return id.replaceAll("~", "~~").replaceAll("\0", "~0").replaceAll("�", "~r").replaceAll("/", "\0");
}
function hydrateSsrMatchId(id) {
	return id.replaceAll("\0", "/").replaceAll("�", "/").replace(/~([~0r])/g, (_, code) => code === "0" ? "\0" : code === "r" ? "�" : code);
}
//#endregion
exports.dehydrateSsrMatchId = dehydrateSsrMatchId;
exports.hydrateSsrMatchId = hydrateSsrMatchId;

//# sourceMappingURL=ssr-match-id.cjs.map