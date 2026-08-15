let node_fs_promises = require("node:fs/promises");
let pathe = require("pathe");
let tinyglobby = require("tinyglobby");
//#region src/copy-files-plugin.ts
function copyFilesPlugin({ fromDir, toDir, pattern = "**" }) {
	return {
		name: "copy-files",
		async writeBundle() {
			const entries = await (0, tinyglobby.glob)(pattern, { cwd: fromDir });
			if (entries.length === 0) throw new Error(`No files found matching pattern "${pattern}" in directory "${fromDir}"`);
			for (const entry of entries) {
				const srcPath = (0, pathe.join)(fromDir, entry);
				const destPath = (0, pathe.join)(toDir, entry);
				await (0, node_fs_promises.mkdir)((0, pathe.dirname)(destPath), { recursive: true });
				await (0, node_fs_promises.copyFile)(srcPath, destPath);
			}
		}
	};
}
//#endregion
exports.copyFilesPlugin = copyFilesPlugin;

//# sourceMappingURL=copy-files-plugin.cjs.map