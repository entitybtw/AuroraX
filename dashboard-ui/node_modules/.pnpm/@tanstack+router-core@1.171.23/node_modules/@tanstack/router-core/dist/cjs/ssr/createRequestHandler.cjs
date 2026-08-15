const require_load_client = require("../load-client.cjs");
const require_headers = require("./headers.cjs");
const require_ssr_server = require("./ssr-server.cjs");
const require_handlerCallback = require("./handlerCallback.cjs");
let _tanstack_history = require("@tanstack/history");
//#region src/ssr/createRequestHandler.ts
const requestWaiters = /* @__PURE__ */ new WeakMap();
function removeRequestWaiter(waiters, index, reject) {
	if (waiters[index] !== reject) return;
	if (index !== waiters.length - 1) {
		waiters[index] = void 0;
		return;
	}
	waiters.pop();
	while (waiters.length && waiters[waiters.length - 1] === void 0) waiters.pop();
}
function waitForRequest(value, signal, onLate) {
	const promise = Promise.resolve(value);
	if (signal.aborted) {
		promise.then(onLate, () => {});
		return Promise.reject(signal.reason);
	}
	return new Promise((resolve, reject) => {
		let waiters = requestWaiters.get(signal);
		let index;
		if (waiters) index = waiters.push(reject) - 1;
		else {
			const newWaiters = [reject];
			waiters = newWaiters;
			index = 0;
			requestWaiters.set(signal, newWaiters);
			signal.addEventListener("abort", () => {
				requestWaiters.delete(signal);
				for (const rejectWaiter of newWaiters) rejectWaiter?.(signal.reason);
				newWaiters.length = 0;
			}, { once: true });
		}
		promise.then((result) => {
			removeRequestWaiter(waiters, index, reject);
			if (signal.aborted) onLate?.(result);
			else resolve(result);
		}, (error) => {
			removeRequestWaiter(waiters, index, reject);
			reject(error);
		});
	});
}
function createRequestHandler({ createRouter, request, getRouterManifest }) {
	return async (cb) => {
		request.signal.throwIfAborted();
		const router = createRouter();
		let responseOwnsCleanup = false;
		try {
			require_ssr_server.attachRouterServerSsrUtils({
				router,
				manifest: await waitForRequest(getRouterManifest?.(), request.signal)
			});
			const { url } = require_ssr_server.getNormalizedURL(request.url, "http://localhost");
			const origin = require_ssr_server.getOrigin(request);
			const history = (0, _tanstack_history.createMemoryHistory)({ initialEntries: [url.href.replace(url.origin, "")] });
			router.update({
				history,
				origin: router.options.origin ?? origin
			});
			await router.load({ _signal: request.signal });
			request.signal.throwIfAborted();
			const result = router._serverResult;
			if (result?.type === "redirect") return result.redirect;
			await waitForRequest(router.serverSsr?.dehydrate(), request.signal);
			request.signal.throwIfAborted();
			const responseHeaders = getRequestHeaders({ router });
			request.signal.throwIfAborted();
			const ssrResponse = require_handlerCallback.bindSsrResponseToRequest(router, await waitForRequest(cb({
				request,
				router,
				responseHeaders
			}), request.signal, (late) => {
				require_handlerCallback.disposeSsrResponseDetached(late, request.signal.reason);
			}), request.signal);
			request.signal.throwIfAborted();
			responseOwnsCleanup = ssrResponse.serverSsrCleanup === "stream";
			return ssrResponse.response;
		} finally {
			if (!responseOwnsCleanup) router.serverSsr?.cleanup();
		}
	};
}
function getRequestHeaders(opts) {
	const matchHeaders = [];
	for (const match of require_load_client._getRenderedMatches(opts.router.stores.matches.get())) matchHeaders.push(match.headers);
	return require_headers.mergeHeaders({ "Content-Type": "text/html; charset=UTF-8" }, ...matchHeaders);
}
//#endregion
exports.createRequestHandler = createRequestHandler;
exports.waitForRequest = waitForRequest;

//# sourceMappingURL=createRequestHandler.cjs.map