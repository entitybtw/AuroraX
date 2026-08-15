import { copyFile, mkdir } from "node:fs/promises";
import { dirname, join } from "pathe";
import { glob } from "tinyglobby";
//#region src/copy-files-plugin.ts
function copyFilesPlugin({ fromDir, toDir, pattern = "**" }) {
	return {
		name: "copy-files",
		async writeBundle() {
			const entries = await glob(pattern, { cwd: fromDir });
			if (entries.length === 0) throw new Error(`No files found matching pattern "${pattern}" in directory "${fromDir}"`);
			for (const entry of entries) {
				const srcPath = join(fromDir, entry);
				const destPath = join(toDir, entry);
				await mkdir(dirname(destPath), { recursive: true });
				await copyFile(srcPath, destPath);
			}
		}
	};
}
//#endregion
export { copyFilesPlugin };

//# sourceMappingURL=copy-files-plugin.js.map