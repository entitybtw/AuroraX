import { _getRenderedMatches } from "../load-client.js";
import { mergeHeaders } from "./headers.js";
import { attachRouterServerSsrUtils, getNormalizedURL, getOrigin } from "./ssr-server.js";
import { bindSsrResponseToRequest, disposeSsrResponseDetached } from "./handlerCallback.js";
import { createMemoryHistory } from "@tanstack/history";
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
			attachRouterServerSsrUtils({
				router,
				manifest: await waitForRequest(getRouterManifest?.(), request.signal)
			});
			const { url } = getNormalizedURL(request.url, "http://localhost");
			const origin = getOrigin(request);
			const history = createMemoryHistory({ initialEntries: [url.href.replace(url.origin, "")] });
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
			const ssrResponse = bindSsrResponseToRequest(router, await waitForRequest(cb({
				request,
				router,
				responseHeaders
			}), request.signal, (late) => {
				disposeSsrResponseDetached(late, request.signal.reason);
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
	for (const match of _getRenderedMatches(opts.router.stores.matches.get())) matchHeaders.push(match.headers);
	return mergeHeaders({ "Content-Type": "text/html; charset=UTF-8" }, ...matchHeaders);
}
//#endregion
export { createRequestHandler, waitForRequest };

//# sourceMappingURL=createRequestHandler.js.map