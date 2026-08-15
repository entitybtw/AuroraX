const require_runtime = require("./_virtual/_rolldown/runtime.cjs");
let ansis = require("ansis");
ansis = require_runtime.__toESM(ansis, 1);
let diff = require("diff");
//#region src/logger.ts
function logDiff(oldStr, newStr) {
	const differences = (0, diff.diffWords)(oldStr, newStr);
	let output = "";
	let unchangedLines = "";
	function processUnchangedLines(lines) {
		const lineArray = lines.split("\n");
		if (lineArray.length > 4) return [
			ansis.default.dim(lineArray[0]),
			ansis.default.dim(lineArray[1]),
			"",
			ansis.default.dim.bold(`... (${lineArray.length - 4} lines) ...`),
			"",
			ansis.default.dim(lineArray[lineArray.length - 2]),
			ansis.default.dim(lineArray[lineArray.length - 1])
		].join("\n");
		return ansis.default.dim(lines);
	}
	differences.forEach((part, index) => {
		const nextPart = differences[index + 1];
		if (part.added) {
			if (unchangedLines) {
				output += processUnchangedLines(unchangedLines);
				unchangedLines = "";
			}
			output += ansis.default.green.bold(part.value);
			if (nextPart?.removed) output += " ";
		} else if (part.removed) {
			if (unchangedLines) {
				output += processUnchangedLines(unchangedLines);
				unchangedLines = "";
			}
			output += ansis.default.red.bold(part.value);
			if (nextPart?.added) output += " ";
		} else unchangedLines += part.value;
	});
	if (unchangedLines) output += processUnchangedLines(unchangedLines);
	if (output) {
		console.log("\nDiff:");
		console.log(output + "\n\n");
	} else console.log("No changes");
}
//#endregion
exports.logDiff = logDiff;

//# sourceMappingURL=logger.cjs.map