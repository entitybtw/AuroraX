//#region src/await-signal.ts
function waitForReason(value, signal, onLate) {
	const promise = Promise.resolve(value);
	if (signal.aborted) {
		if (!onLate) return Promise.race([Promise.reject(signal.reason), promise]);
		promise.then(onLate, () => {});
		return Promise.reject(signal.reason);
	}
	return new Promise((resolve, reject) => {
		const abort = () => reject(signal.reason);
		signal.addEventListener("abort", abort, { once: true });
		promise.then((result) => {
			if (signal.aborted) onLate?.(result);
			else resolve(result);
		}, reject).finally(() => signal.removeEventListener("abort", abort));
	});
}
//#endregion
export { waitForReason };

//# sourceMappingURL=await-signal.js.map