//#region src/ssr/handlerCallback.ts
function isSsrResponse(value) {
	return typeof value === "object" && value !== null && "response" in value && "serverSsrCleanup" in value;
}
function normalizeSsrResponse(result) {
	return isSsrResponse(result) ? result : {
		response: result,
		serverSsrCleanup: "none"
	};
}
function disposeSsrResponse(response, reason) {
	if (response.serverSsrCleanup !== "stream") return Promise.resolve();
	try {
		return Promise.resolve(response.dispose(reason));
	} catch (error) {
		return Promise.reject(error);
	}
}
function disposeSsrResponseDetached(result, reason, onError = console.error) {
	const ssrResponse = normalizeSsrResponse(result);
	if (ssrResponse.serverSsrCleanup === "stream") {
		disposeSsrResponse(ssrResponse, reason).catch(onError);
		return;
	}
	if (ssrResponse.response.body) try {
		ssrResponse.response.body.cancel(reason).catch(onError);
	} catch (error) {
		onError(error);
	}
}
function createSsrStreamResponse(router, response) {
	if (!response.body) throw new Error("Invariant failed: SSR stream response requires a body");
	let disposed = false;
	return {
		response,
		serverSsrCleanup: "stream",
		async dispose(reason) {
			if (disposed) return;
			disposed = true;
			router.serverSsr?.cleanup();
			try {
				await response.body.cancel(reason);
			} catch {}
		}
	};
}
function bindSsrResponseToRequest(router, result, signal) {
	const ssrResponse = normalizeSsrResponse(result);
	if (ssrResponse.serverSsrCleanup !== "stream") {
		if (signal.aborted) disposeSsrResponseDetached(result, signal.reason);
		return ssrResponse;
	}
	const failed = (error) => {
		router?.serverSsr?.cleanup();
		console.error(error);
	};
	const abort = () => {
		disposeSsrResponseDetached(ssrResponse, signal.reason, failed);
	};
	if (signal.aborted) {
		abort();
		return ssrResponse;
	}
	signal.addEventListener("abort", abort, { once: true });
	router?.serverSsr?.onCleanup(() => {
		signal.removeEventListener("abort", abort);
	});
	return ssrResponse;
}
async function replaceSsrResponse(result, response, reason) {
	await disposeSsrResponse(normalizeSsrResponse(result), reason);
	return {
		response,
		serverSsrCleanup: "none"
	};
}
async function stripSsrResponseBody(result, reason) {
	const ssrResponse = normalizeSsrResponse(result);
	await disposeSsrResponse(ssrResponse, reason);
	return {
		response: new Response(null, ssrResponse.response),
		serverSsrCleanup: "none"
	};
}
function defineHandlerCallback(handler) {
	return handler;
}
//#endregion
exports.bindSsrResponseToRequest = bindSsrResponseToRequest;
exports.createSsrStreamResponse = createSsrStreamResponse;
exports.defineHandlerCallback = defineHandlerCallback;
exports.disposeSsrResponse = disposeSsrResponse;
exports.disposeSsrResponseDetached = disposeSsrResponseDetached;
exports.isSsrResponse = isSsrResponse;
exports.normalizeSsrResponse = normalizeSsrResponse;
exports.replaceSsrResponse = replaceSsrResponse;
exports.stripSsrResponseBody = stripSsrResponseBody;

//# sourceMappingURL=handlerCallback.cjs.map