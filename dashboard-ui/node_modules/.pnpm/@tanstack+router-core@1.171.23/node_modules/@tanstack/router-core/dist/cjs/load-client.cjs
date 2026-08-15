const require_not_found = require("./not-found.cjs");
const require_redirect = require("./redirect.cjs");
const require_ssr_match_id = require("./ssr/ssr-match-id.cjs");
const require_router = require("./router.cjs");
//#region src/load-client.ts
function replaceRouteChunk(route, lazyFn) {
	route.lazyFn = lazyFn ?? route.lazyFn;
	route._lazy = void 0;
}
function preloadComponent(route, type) {
	return route.options[type]?.preload?.();
}
function loadComponents(route, onPendingReady) {
	const component = preloadComponent(route, "component");
	const pending = preloadComponent(route, "pendingComponent");
	const pendingReady = onPendingReady && pending ? pending.then(onPendingReady) : pending;
	if (onPendingReady && !pending) onPendingReady();
	if (component && pendingReady) return Promise.all([component, pendingReady]).then(() => {});
	return component ?? pendingReady;
}
function loadRouteChunk(route, componentType, onPendingReady) {
	const afterLazy = () => componentType === false ? void 0 : componentType ? preloadComponent(route, componentType) : loadComponents(route, onPendingReady);
	const current = route._lazy;
	if (current) return current === true ? afterLazy() : current.then(afterLazy);
	if (!route.lazyFn) return afterLazy();
	const promise = route.lazyFn().then((lazyRoute) => {
		if (process.env.NODE_ENV === "production" || route._lazy === promise) {
			const { id: _id, ...options } = lazyRoute.options;
			Object.assign(route.options, options);
			route._lazy = true;
		}
	}, (error) => {
		if (process.env.NODE_ENV === "production" || route._lazy === promise) route._lazy = void 0;
		throw error;
	});
	route._lazy = promise;
	return promise.then(afterLazy);
}
/** Return the structural lane through the first terminal render boundary. */
function _getRenderedMatches(matches) {
	const end = matches.findIndex((match) => match.status !== "success" || match._notFound) + 1;
	return end && end < matches.length ? matches.slice(0, end) : matches;
}
/** Return the lane whose document assets belong to the current presentation. */
function _getAssetMatches(matches) {
	let end = matches.length;
	for (let index = 0; index < end; index++) {
		const match = matches[index];
		if (match._assetEnd !== void 0) {
			end = Math.min(end, Math.max(index + 1, match._assetEnd));
			continue;
		}
		if (match.status !== "success" || match._notFound) {
			end = index + 1;
			break;
		}
	}
	return end < matches.length ? matches.slice(0, end) : matches;
}
const SUCCESS = 0;
const ERROR = 1;
const NOT_FOUND = 2;
const REDIRECTED = 3;
const CANCELED = 4;
function isControl(result) {
	return typeof result[0] === "number";
}
function waitFor(value, signal) {
	if (signal.aborted) return Promise.race([Promise.reject(signal), value]);
	return new Promise((resolve, reject) => {
		const abort = () => reject(signal);
		signal.addEventListener("abort", abort, { once: true });
		Promise.resolve(value).then(resolve, reject).finally(() => signal.removeEventListener("abort", abort));
	});
}
function getRoute(router, match) {
	return router.routesById[match.routeId];
}
function normalize(value, rejected, routeId) {
	if (require_redirect.isRedirect(value)) return [REDIRECTED, value];
	if (require_not_found.isNotFound(value)) {
		value.routeId ||= routeId;
		return [NOT_FOUND, value];
	}
	if (rejected && typeof value?.then === "function") value = new Error("A Promise was thrown", { cause: value });
	return rejected ? [ERROR, value] : [SUCCESS, value];
}
function normalizeError(route, cause) {
	let outcome = normalize(cause, true, route.id);
	if (outcome[0] !== ERROR) return outcome;
	try {
		route.options.onError?.(outcome[1]);
	} catch (onErrorCause) {
		outcome = normalize(onErrorCause, true, route.id);
	}
	return outcome;
}
function normalizeLaneError(route, cause, options) {
	if (options[0].signal.aborted || !options[2]()) {
		options[0].abort();
		return [CANCELED];
	}
	return normalizeError(route, cause);
}
function navigateFrom(router, location) {
	return (opts) => router.navigate({
		...opts,
		_fromLocation: location
	});
}
async function contextualize(router, lane, options, end, planSuccessfulLane, retainedEnd) {
	const [location, matches] = lane;
	const signal = options[0].signal;
	const preload = !!options[4];
	for (let index = options[7] ?? 0; index < end; index++) {
		const match = matches[index];
		const route = getRoute(router, match);
		match.abortController = options[0];
		const parentContext = matches[index - 1]?.context ?? router.options.context ?? {};
		const common = {
			params: match.params,
			location,
			navigate: navigateFrom(router, location),
			buildLocation: router.buildLocation,
			cause: preload ? "preload" : match.cause,
			abortController: options[0],
			preload,
			matches,
			routeId: route.id
		};
		let context = parentContext;
		try {
			let routeContext = match._ctx;
			if (!routeContext && route.options.context) routeContext = match._ctx = route.options.context({
				...common,
				deps: match.loaderDeps,
				context: parentContext
			}) || {};
			context = {
				...parentContext,
				...routeContext
			};
			match.context = context;
		} catch (cause) {
			releaseFlight(router, match);
			return [index, normalizeLaneError(route, cause, options)];
		}
		if (signal.aborted || !options[2]()) {
			options[0].abort();
			return [index, [CANCELED]];
		}
		const validationError = match.paramsError ?? match.searchError;
		if (validationError !== void 0) {
			releaseFlight(router, match);
			return [index, normalizeLaneError(route, validationError, options)];
		}
		const beforeLoad = route.options.beforeLoad;
		if (!beforeLoad) continue;
		const beforeLoadContext = {
			...common,
			search: match.search,
			context,
			...router.options.additionalContext
		};
		const previousStatus = match.status;
		if (previousStatus === "success" && index >= retainedEnd) match.status = "pending";
		options[8]?.();
		try {
			setFetching(router, match, "beforeLoad", options[0]);
			const result = await waitFor(beforeLoad(beforeLoadContext), signal);
			if (!options[2]()) {
				options[0].abort();
				return [index, [CANCELED]];
			}
			const outcome = normalize(result, false, route.id);
			if (outcome[0] !== SUCCESS) {
				releaseFlight(router, match);
				return [index, outcome];
			}
			match.context = {
				...context,
				...result
			};
		} catch (cause) {
			releaseFlight(router, match);
			return [index, normalizeLaneError(route, cause, options)];
		} finally {
			if (previousStatus === "success" && match.status === "pending") match.status = "success";
			setFetching(router, match, false, options[0]);
		}
	}
	planSuccessfulLane();
}
function releaseOwnedFlight(router, match, flight) {
	if (!flight || --flight[2]) return;
	if (router._flights?.get(match.id) === flight) {
		const current = router._tx;
		if (current && !current[0].signal.aborted && !(process.env.NODE_ENV !== "production" && current[6]) && !current[3].includes(match) && current[3].some((candidate) => candidate.id === match.id) && current[3].some((candidate) => candidate.isFetching === "beforeLoad")) return;
		router._flights.delete(match.id);
	}
	return flight[1];
}
function releaseFlight(router, match) {
	const flight = match._flight;
	match._flight = void 0;
	releaseOwnedFlight(router, match, flight)?.abort();
}
/**
* Not passing in a `next` ownership recipient
* is equivalent to discarding the match resources
*/
function transferMatchResources(router, previous, next, deferSameIdFlight) {
	const abort = [];
	for (const match of previous) if (!next?.includes(match)) {
		const flight = match._flight;
		match._flight = void 0;
		if (deferSameIdFlight && flight?.[2] === 1 && router._flights?.get(match.id) === flight && !(process.env.NODE_ENV !== "production" && router._tx?.[6]) && next?.some((candidate) => candidate.id === match.id)) flight[2] = 0;
		else {
			const controller = releaseOwnedFlight(router, match, flight);
			if (controller) abort.push(controller);
		}
	}
	for (const controller of abort) controller.abort();
}
function acquireMatchResources(matches) {
	for (const match of matches) {
		const flight = match._flight;
		if (flight) flight[2]++;
	}
}
function setFetching(router, match, value, owner) {
	match.isFetching = value;
	if (owner && router._tx?.[0] !== owner) return;
	const store = router.stores.byRoute.get(match.routeId);
	const presented = store?.get();
	if (presented?.id === match.id) store.set({
		...presented,
		isFetching: value
	});
}
function getLoaderContext(router, lane, match, route, controller, parentMatchPromise, preload) {
	const location = lane[0];
	return {
		params: match.params,
		location,
		navigate: navigateFrom(router, location),
		cause: preload ? "preload" : match.cause,
		abortController: controller,
		preload,
		deps: match.loaderDeps,
		parentMatchPromise,
		context: match.context,
		route,
		...router.options.additionalContext
	};
}
async function loadResource(router, lane, match, route, loader, parentMatchPromise, preload, owner) {
	const signal = owner.signal;
	if (signal.aborted) return [CANCELED];
	if (!loader) return [SUCCESS, void 0];
	let flight = match._flight;
	setFetching(router, match, "loader", owner);
	try {
		if (!flight) {
			const controller = new AbortController();
			flight = [
				Promise.resolve().then(() => loader(getLoaderContext(router, lane, match, route, controller, parentMatchPromise, preload))).then((value) => normalize(value, false, route.id), (cause) => normalize(cause, true, route.id)).then((result) => {
					if (result[0] !== SUCCESS && router._flights?.get(match.id) === flight) {
						router._flights.delete(match.id);
						if (!flight[2]) controller.abort();
					}
					return result[0] === ERROR && flight[2] ? normalizeError(route, result[1]) : result;
				}),
				controller,
				1
			];
			(router._flights ??= /* @__PURE__ */ new Map()).set(match.id, flight);
		}
		match._flight = flight;
		match.abortController = flight[1];
		return await waitFor(flight[0], signal);
	} catch (cause) {
		if (cause !== signal) throw cause;
		releaseFlight(router, match);
		return [CANCELED];
	} finally {
		setFetching(router, match, false, owner);
	}
}
function settleInto(match, result, preload) {
	if (result[0] === SUCCESS) {
		match.loaderData = result[1];
		match.error = void 0;
		match.status = "success";
		match.invalid = false;
		match.updatedAt = Date.now();
		match.preload = preload;
	} else if (result[0] !== REDIRECTED) {
		match.status = "success";
		match.error = void 0;
		match.invalid = true;
	}
}
function cacheLoaderMatch(router, match, planned) {
	const current = router._cache.get(match.id);
	if (current !== planned || router._committed.some((candidate) => candidate.id === match.id && candidate._flight === match._flight)) return;
	const cached = {
		...match,
		_notFound: void 0,
		context: {}
	};
	if (cached._flight) cached._flight[2]++;
	router._cache.set(match.id, cached);
	if (current) releaseFlight(router, current);
}
function getParentSnapshot(match, outcome) {
	if (outcome[0] === ERROR || outcome[0] === NOT_FOUND) return {
		...match,
		status: outcome[0] === ERROR ? "error" : "notFound",
		error: outcome[1],
		_flight: void 0
	};
	return match;
}
function createLoaderTask(router, lane, index, tasks, semanticParent, options, retainedEnd) {
	const match = lane[1][index];
	const route = getRoute(router, match);
	const preload = !!options[4];
	const plannedCacheMatch = router._cache.get(match.id);
	let configured;
	let reload = false;
	let reloadFailure;
	try {
		if (match.status === "success") {
			configured = route.options.shouldReload;
			if (typeof configured === "function") configured = configured(getLoaderContext(router, lane, match, route, options[0], semanticParent, preload));
			if (!options[2]()) {
				options[0].abort();
				reloadFailure = [CANCELED];
			}
		}
		if (!reloadFailure) if (match.status !== "success") reload = true;
		else {
			const staleAge = options[4] || match.preload ? route.options.preloadStaleTime ?? router.options.defaultPreloadStaleTime ?? 3e4 : route.options.staleTime ?? router.options.defaultStaleTime ?? 0;
			reload = !!(match.invalid || configured || configured === void 0 && Date.now() - match.updatedAt >= staleAge && (options[6] || match.cause === "enter" || options[3].some((candidate) => candidate.routeId === match.routeId && candidate.id !== match.id)));
		}
	} catch (cause) {
		match.invalid = true;
		releaseFlight(router, match);
		reloadFailure = normalizeLaneError(route, cause, options);
	}
	const routeLoader = route.options.loader;
	const loader = typeof routeLoader === "function" ? routeLoader : routeLoader?.handler;
	let donor = (!preload || route.options.preload !== false) && routeLoader && !(process.env.NODE_ENV !== "production" && router._tx?.[6]) ? router._flights?.get(match.id) : void 0;
	if (donor === match._flight || reloadFailure) donor = void 0;
	else if (donor && !reload && !preload && configured === void 0) reload = true;
	else if (!reload) donor = void 0;
	const background = !!(routeLoader && reload && match.status === "success" && !preload && !options[5] && ((typeof routeLoader === "function" ? void 0 : routeLoader?.staleReloadMode) ?? router.options.defaultStaleReloadMode) !== "blocking");
	const loaded = reload && (!preload || route.options.preload !== false);
	const blocking = loaded && !background && (match.status !== "success" || !!routeLoader);
	const onLazyReady = route.lazyFn && route._lazy !== true ? options[8] : void 0;
	if (loaded && !routeLoader) {
		match.invalid = false;
		match.updatedAt = Date.now();
	}
	if (donor) donor[2]++;
	if (blocking) {
		const acceptedFlight = match._flight;
		match._flight = donor;
		releaseOwnedFlight(router, match, acceptedFlight)?.abort();
		if (match.status === "success" && index >= retainedEnd) match.status = "pending";
		options[8]?.();
	}
	if (!loaded) match.isFetching = false;
	const outcome = (reloadFailure ? Promise.resolve(reloadFailure) : !blocking ? Promise.resolve([SUCCESS, match.loaderData]) : loadResource(router, lane, match, route, loader, semanticParent, preload, options[0])).then((result) => {
		if (blocking) {
			settleInto(match, result, preload);
			if (result[0] === SUCCESS) {
				if (routeLoader && !options[0].signal.aborted && !(process.env.NODE_ENV !== "production" && !preload && router._tx?.[6])) cacheLoaderMatch(router, match, plannedCacheMatch);
				match.status = "pending";
			}
		}
		return result;
	});
	const chunkFailure = waitFor(Promise.resolve().then(() => loadRouteChunk(route, void 0, onLazyReady)), options[0].signal).then(() => void 0, (cause) => [index, normalizeLaneError(route, cause, options)]).then((failure) => outcome.then((result) => {
		if (blocking && !failure && result[0] === SUCCESS && match.status === "pending" && options[2]()) {
			match.status = "success";
			options[8]?.();
		}
		return failure;
	}));
	tasks.push([
		index,
		outcome,
		chunkFailure
	]);
	if (!background) return outcome.then((result) => getParentSnapshot(match, result));
	const candidate = {
		...match,
		status: "pending",
		preload: false,
		_flight: donor
	};
	match.invalid = false;
	match.isFetching = "loader";
	const backgroundOutcome = loadResource(router, lane, candidate, route, loader, semanticParent, false, options[0]).then((result) => {
		match.isFetching = false;
		settleInto(candidate, result, false);
		return result;
	});
	(lane[2] ??= []).push([
		index,
		backgroundOutcome,
		chunkFailure,
		candidate
	]);
	return backgroundOutcome.then((result) => getParentSnapshot(candidate, result));
}
async function getNotFoundBoundary(router, matches, indexed, signal, fallback = 0) {
	const cause = indexed?.[1][1];
	let index = cause?.routeId ? matches.findIndex((match) => match.routeId === cause.routeId) : indexed?.[0] ?? matches.length - 1;
	if (index < 0) index = 0;
	for (let i = index; i >= 0; i--) {
		const route = getRoute(router, matches[i]);
		const loading = loadRouteChunk(route, false);
		if (loading) try {
			await waitFor(loading, signal);
		} catch (cause) {
			if (cause === signal) throw cause;
		}
		if (route.options.notFoundComponent) return i;
	}
	return cause?.routeId ? index : fallback;
}
function discardBackground(router, lane) {
	if (lane[2]) {
		transferMatchResources(router, lane[2].map((task) => task[3]));
		lane[2] = void 0;
	}
}
async function settleTasks(tasks, serialFailure, redirectTasks, gate) {
	let loaderFailure;
	try {
		await Promise.all(tasks.map((task) => task[1].then(async (outcome) => {
			const taskIndex = task[0];
			if (gate && taskIndex >= await gate) return;
			if (outcome[0] >= REDIRECTED) throw [taskIndex, outcome];
			if (!loaderFailure && outcome[0] !== SUCCESS) {
				loaderFailure = [taskIndex, outcome];
				await Promise.all((redirectTasks ?? []).map((nextTask) => {
					if (nextTask[0] <= taskIndex) return;
					return nextTask[1].then((nextOutcome) => {
						if (nextOutcome[0] === REDIRECTED) throw [nextTask[0], nextOutcome];
					});
				}));
			}
		})));
	} catch (cause) {
		return cause;
	}
	return serialFailure ?? loaderFailure;
}
async function reduceLane(router, lane, tasks, controller, redirects, settlement, onReady) {
	const matches = lane[1];
	let failure = await settlement;
	let redirectLimitExceeded = false;
	const plannedBoundary = matches.findIndex((match) => match._notFound);
	const boundaryOf = (found) => found[1][0] === NOT_FOUND ? getNotFoundBoundary(router, matches, found, controller.signal) : found[0];
	let readinessEnd = plannedBoundary < 0 ? matches.length : plannedBoundary;
	if ((failure?.[1][0] ?? 0) >= REDIRECTED) readinessEnd = 0;
	else if (failure) {
		readinessEnd = failure[2] ??= await boundaryOf(failure);
		for (const task of tasks) {
			if (task[0] >= readinessEnd) break;
			const outcome = await task[1];
			if (outcome[0] !== SUCCESS && outcome[0] < REDIRECTED && !("loaderData" in matches[task[0]])) {
				failure = [task[0], outcome];
				readinessEnd = failure[2] = await boundaryOf(failure);
				break;
			}
		}
	}
	for (const task of tasks) {
		if (task[0] >= readinessEnd) break;
		const chunkFailure = await task[2];
		if (!chunkFailure) continue;
		failure = chunkFailure;
		break;
	}
	if ((failure?.[1][0] ?? 0) >= REDIRECTED) {
		const outcome = failure[1];
		if (outcome[0] !== REDIRECTED || outcome[1].options.reloadDocument || redirects < 20) {
			discardBackground(router, lane);
			return outcome;
		}
		redirectLimitExceeded = true;
		failure = [0, [ERROR, /* @__PURE__ */ new Error("Too many redirects")]];
	}
	const boundary = failure ? failure[2] ?? await boundaryOf(failure) : plannedBoundary;
	if (boundary >= 0) {
		const outcome = failure?.[1];
		const kind = outcome?.[0];
		const match = matches[boundary];
		const cause = outcome?.[1];
		const install = () => {
			if (outcome) {
				match._notFound = void 0;
				if (kind === ERROR) match.status = "error";
				else {
					cause.routeId = match.routeId;
					if (match.routeId === router.routeTree.id) {
						match.status = "success";
						match._notFound = true;
					} else match.status = "notFound";
				}
				match.error = cause;
				match.isFetching = false;
			}
		};
		install();
		const route = getRoute(router, match);
		try {
			await waitFor(outcome ? Promise.resolve().then(() => loadRouteChunk(route, kind === ERROR ? "errorComponent" : "notFoundComponent")) : Promise.all([loadRouteChunk(route), loadRouteChunk(route, "notFoundComponent")]), controller.signal);
		} catch (cause) {
			if (cause === controller.signal) {
				discardBackground(router, lane);
				return [CANCELED];
			}
		}
		if (!outcome) {
			match.status = "success";
			onReady?.();
		} else if (redirectLimitExceeded) {
			controller.abort();
			await Promise.all([
				...tasks.map((task) => task[1]),
				...tasks.map((task) => task[2]),
				...(lane[2] ?? []).map((task) => task[1])
			]);
			discardBackground(router, lane);
			transferMatchResources(router, matches);
			install();
		}
	}
	return lane;
}
async function projectLane(router, lane, signal, start = 0, end = lane[1].length) {
	const matches = lane[1];
	for (let index = start; index < end; index++) {
		const match = matches[index];
		const routeOptions = getRoute(router, match).options;
		if (routeOptions.head || routeOptions.scripts) try {
			const context = {
				ssr: router.options.ssr,
				matches,
				match,
				params: match.params,
				loaderData: match.loaderData
			};
			const [head, scripts] = await waitFor(Promise.all([routeOptions.head?.(context), routeOptions.scripts?.(context)]), signal);
			match.meta = head?.meta;
			match.links = head?.links;
			match.headScripts = head?.scripts;
			match.styles = head?.styles;
			match.scripts = scripts;
		} catch (cause) {
			if (cause === signal) break;
			console.error(cause);
		}
		if (match.status !== "success" || match._notFound) break;
	}
	return lane;
}
async function executeClientLane(router, location, matches, options) {
	const matched = [location, matches];
	const presented = router.stores.matches.get();
	let plannedBoundary = matches.findIndex((match) => match._notFound);
	if (router.options.notFoundMode !== "root" && plannedBoundary >= 0) {
		const boundary = await getNotFoundBoundary(router, matched[1], void 0, options[0].signal, plannedBoundary);
		if (boundary !== plannedBoundary) {
			matches[plannedBoundary]._notFound = void 0;
			matches[boundary]._notFound = true;
		}
		plannedBoundary = boundary;
	}
	let end = plannedBoundary < 0 ? matches.length : plannedBoundary + 1;
	let retainedEnd = 0;
	while (retainedEnd < end && retainedEnd !== plannedBoundary) {
		const match = matches[retainedEnd];
		const committed = options[3][retainedEnd];
		const visible = presented[retainedEnd];
		if (committed?.id !== match.id || committed.status !== "success" || committed._notFound || match.preload || visible?.id !== match.id || visible.status !== "success" || visible._notFound) break;
		retainedEnd++;
	}
	const tasks = [];
	const start = options[7] ?? 0;
	let semanticParent = start ? Promise.resolve(matched[1][start - 1]) : void 0;
	const planSuccessfulLane = () => {
		for (let index = start; index < end; index++) {
			if (options[0].signal.aborted) break;
			semanticParent = createLoaderTask(router, matched, index, tasks, semanticParent, options, retainedEnd);
		}
	};
	const failure = await contextualize(router, matched, options, end, planSuccessfulLane, retainedEnd);
	if (failure) {
		options[5] = true;
		end = failure[0];
		if (failure[1][0] === NOT_FOUND) {
			failure[2] = await getNotFoundBoundary(router, matched[1], failure, options[0].signal);
			end = Math.min(end, failure[2] + 1);
		} else if (failure[1][0] >= REDIRECTED) end = 0;
		planSuccessfulLane();
	}
	if (options[2]() && !options[4]) {
		const abort = [];
		for (const [id, flight] of router._flights ?? []) if (!flight[2]) {
			router._flights.delete(id);
			abort.push(flight[1]);
		}
		for (const controller of abort) controller.abort();
	}
	let reduced;
	try {
		const reduction = reduceLane(router, matched, tasks, options[0], options[1], settleTasks(tasks, failure, matched[2]), options[8]);
		if (matched[2]?.length) matched[3] = settleTasks(matched[2], void 0, void 0, reduction.then((foreground) => isControl(foreground) ? 0 : _getRenderedMatches(foreground[1]).length, () => 0));
		reduced = await reduction;
	} catch (cause) {
		discardBackground(router, matched);
		throw cause;
	}
	if (isControl(reduced)) return reduced;
	return projectLane(router, reduced, options[0].signal, options[7] === reduced[1].length ? options[7] : 0);
}
/**
* Waits for `pendingMs`, then presents the complete lane. Rendering applies the
* selected boundary cutoff while retaining every match's structural state.
* A replacement load for the same match keeps the timer; choosing a different
* match resets it. `pendingMinMs` starts after the fallback renders.
*/
function offerPending(router, tx) {
	if (router._tx !== tx) return;
	let session = router._pending;
	let tookOver = false;
	const sessionMatchId = session?.[0][3][session[1]]?.id;
	if (session?.[0] !== tx) if (session && tx[3][session[1]]?.id === sessionMatchId) {
		session[0] = tx;
		tookOver = true;
	} else {
		clearTimeout(session?.[3]);
		router._pending = session = void 0;
	}
	const matches = tx[3];
	const presented = router.stores.matches.get();
	let boundary = -1;
	let delay;
	let min;
	let component;
	let presentedPending = false;
	for (let index = 0; index < matches.length; index++) {
		const match = matches[index];
		const success = match.status === "success";
		presentedPending = presented[index]?.id === match.id && presented[index]?.status === "pending";
		if (success && !presentedPending) continue;
		const route = getRoute(router, match);
		delay = success && presentedPending || match.invalid ? 0 : route.options.pendingMs ?? router.options.defaultPendingMs;
		component = route.options.pendingComponent ?? router.options.defaultPendingComponent;
		if (!component || typeof delay !== "number" || delay === Infinity) return;
		boundary = index;
		min = route.options.pendingMinMs ?? router.options.defaultPendingMinMs ?? 0;
		break;
	}
	if (boundary < 0) return;
	const matchId = matches[boundary].id;
	if (!session || session[1] !== boundary || sessionMatchId !== matchId) {
		clearTimeout(session?.[3]);
		router._pending = session = [
			tx,
			boundary,
			presentedPending ? Date.now() + min : tx[4] + delay,
			void 0,
			presentedPending ? Promise.resolve(true) : void 0,
			component
		];
	}
	if (session[4] && !tookOver && session[5] === component) return;
	session[5] = component;
	if (!session[4]) {
		clearTimeout(session[3]);
		const remaining = session[2] - Date.now();
		if (remaining > 0) {
			session[3] = setTimeout(() => offerPending(router, tx), remaining);
			return;
		}
		session[2] = 0;
	}
	const offered = matches.map((match) => ({
		...match,
		_flight: void 0
	}));
	offered[boundary].status = "pending";
	const ack = router.startTransition(() => router.stores.setMatches(offered), offered).then((rendered) => {
		if (rendered && router._pending === session && session[4] === ack && !session[2]) session[2] = Date.now() + min;
		return rendered;
	});
	session[4] = ack;
}
/**
* Cancels pending UI timing when its load ends. The ownership check prevents
* an older, superseded load from clearing pending UI that a newer load took over.
*/
function finishPending(router, tx) {
	const session = router._pending;
	if (session?.[0] === tx) {
		clearTimeout(session[3]);
		router._pending = void 0;
	}
}
function publishMatches(router, matches) {
	router._committed = matches;
	router.stores.setMatches(matches);
}
function discardLane(router, lane) {
	transferMatchResources(router, lane[1]);
	discardBackground(router, lane);
}
function commitMatches(router, tx, matches, resolvedPrefix) {
	const previous = router._committed;
	const previousCached = router._cache;
	for (const match of matches) {
		match.preload = false;
		if (resolvedPrefix) match._assetEnd = void 0;
	}
	const cut = _getRenderedMatches(matches).length;
	const cached = /* @__PURE__ */ new Map();
	const now = Date.now();
	for (const match of [...previous, ...previousCached.values()]) {
		if (match.status !== "success" || matches.some((candidate, index) => candidate.id === match.id && (index < cut || candidate.status === "success"))) continue;
		const route = getRoute(router, match);
		if (!route.options.loader || now - match.updatedAt >= (match.preload ? route.options.preloadGcTime ?? router.options.defaultPreloadGcTime ?? 3e5 : route.options.gcTime ?? router.options.defaultGcTime ?? 3e5)) continue;
		cached.set(match.id, previousCached.get(match.id) === match ? match : {
			...match,
			_flight: void 0,
			isFetching: false,
			context: {}
		});
	}
	tx[3] = [];
	router._cache = cached;
	publishMatches(router, matches);
	transferMatchResources(router, [...previousCached.values(), ...previous], [...matches, ...cached.values()]);
	require_router.runRouteLifecycle(router, previous, matches, () => router._tx === tx);
}
function commitRefreshMatches(router, tx, matches, checkpoint) {
	const previous = router._committed;
	const previousCached = router._cache;
	for (const match of matches) match.preload = false;
	const cached = /* @__PURE__ */ new Map();
	tx[3] = [];
	router._cache = cached;
	checkpoint.previousMatches = previous;
	checkpoint.previousCache = previousCached;
	checkpoint.published = true;
	publishMatches(router, matches);
	if (!checkpoint.published || router._tx !== tx) return;
	require_router.runRouteLifecycle(router, previous, matches, () => router._tx === tx);
}
function settlePublication(router, checkpoint) {
	if (!checkpoint.published) return;
	checkpoint.published = false;
	transferMatchResources(router, [...checkpoint.previousCache.values(), ...checkpoint.previousMatches], [...router._cache.values(), ...router._committed]);
}
function rollbackPublication(router, tx, lane, checkpoint) {
	if (!checkpoint.published || router._tx !== tx || router._committed !== lane[1]) {
		settlePublication(router, checkpoint);
		return false;
	}
	const discarded = [...router._cache.values(), ...router._committed];
	const restored = [...checkpoint.previousCache.values(), ...checkpoint.previousMatches];
	router._cache = checkpoint.previousCache;
	router._committed = checkpoint.previousMatches;
	checkpoint.published = false;
	for (const match of discarded) if (!restored.includes(match) && match._flight && router._flights?.get(match.id) === match._flight) router._flights.delete(match.id);
	finishPending(router, tx);
	router.batch(() => {
		router.stores.status.set("idle");
		router.stores.setMatches(checkpoint.previousPresentation);
	});
	tx[0].abort();
	transferMatchResources(router, discarded, restored);
	discardBackground(router, lane);
	if (router._tx === tx && router._commitPromise === checkpoint.commitPromise) {
		router._commitPromise?.resolve();
		router._commitPromise = void 0;
	}
	return true;
}
async function transitionRefresh(router, tx, lane, changeInfo) {
	const refresh = tx[6];
	const checkpoint = {
		previousMatches: router._committed,
		previousPresentation: refresh[0],
		previousCache: router._cache,
		commitPromise: router._commitPromise,
		published: false
	};
	const commit = () => {
		finishPending(router, tx);
		refresh[2] = rollback;
		commitRefreshMatches(router, tx, lane[1], checkpoint);
		if (!checkpoint.published || router._tx !== tx) return;
		router.emit({
			type: "onLoad",
			...changeInfo
		});
		if (router._tx === tx) router.emit({
			type: "onBeforeRouteMount",
			...changeInfo
		});
	};
	const rollback = () => {
		if (refresh[2] === rollback) refresh[2] = void 0;
		const restored = rollbackPublication(router, tx, lane, checkpoint);
		router._cancelTransition?.();
		return restored;
	};
	try {
		const rendered = await router.startTransition(commit, lane[1]);
		if (refresh[2] === rollback) refresh[2] = void 0;
		if (checkpoint.published) {
			const handoff = refresh[1];
			if (handoff && router._handoff === handoff) handoff[1]();
			if (router._tx === tx) tx[6] = void 0;
		}
		settlePublication(router, checkpoint);
		return rendered;
	} catch (cause) {
		if (rollback()) return;
		throw cause;
	}
}
async function awaitCurrent(router, owner) {
	let current = router._tx;
	while (current && current !== owner) {
		await current[5];
		if (router._tx === current) return;
		current = router._tx;
	}
}
async function followRedirect(router, tx, redirect) {
	await router.navigate({
		...redirect.options,
		replace: true,
		ignoreBlocker: true,
		_redirects: tx[1] + 1
	});
}
function restoreCommitted(router, tx) {
	finishPending(router, tx);
	tx[0].abort();
	transferMatchResources(router, tx[3]);
	tx[3] = [];
	if (router._tx !== tx) return;
	router.batch(() => {
		router.stores.status.set("idle");
		router.stores.setMatches(router._committed);
	});
	if (router._tx === tx) {
		router._commitPromise?.resolve();
		router._commitPromise = void 0;
	}
}
async function runBackground(router, tx, base, tasks, settlement) {
	const next = base.map((match) => ({ ...match }));
	acquireMatchResources(next);
	for (const task of tasks) {
		releaseFlight(router, next[task[0]]);
		next[task[0]] = task[3];
	}
	const lane = [tx[2], next];
	let reduced;
	try {
		reduced = await reduceLane(router, lane, tasks, tx[0], tx[1], settlement);
	} catch (cause) {
		transferMatchResources(router, next);
		throw cause;
	}
	if (isControl(reduced)) {
		transferMatchResources(router, next);
		if (reduced[0] === REDIRECTED && router._tx === tx && router._committed === base) await followRedirect(router, tx, reduced[1]);
		return;
	}
	const projected = await projectLane(router, reduced, tx[0].signal);
	if (router._tx !== tx || router._committed !== base) {
		transferMatchResources(router, projected[1]);
		return;
	}
	for (const match of projected[1]) {
		const cached = router._cache.get(match.id);
		if (cached?._flight && cached._flight === match._flight) {
			router._cache.delete(match.id);
			releaseFlight(router, cached);
		}
	}
	publishMatches(router, projected[1]);
	transferMatchResources(router, base, projected[1]);
}
async function runClientTransaction(router, tx, forceStaleReload, onReady, sync, resolvedPrefix) {
	const options = [
		tx[0],
		tx[1],
		() => router._tx === tx && !!tx[3].length,
		router._committed,
		void 0,
		sync,
		forceStaleReload,
		resolvedPrefix,
		onReady
	];
	const result = await executeClientLane(router, tx[2], tx[3], options);
	if (isControl(result)) {
		if (result[0] === REDIRECTED && router._tx === tx) {
			finishPending(router, tx);
			transferMatchResources(router, tx[3]);
			tx[3] = [];
			if (router._tx === tx) {
				if (process.env.NODE_ENV !== "production" && tx[6]) router._refreshNextLoad = true;
				await followRedirect(router, tx, result[1]);
			}
		} else restoreCommitted(router, tx);
		return;
	}
	const pending = router._pending;
	if (pending?.[0] === tx) {
		/**
		* Loading finished, so cancel any pending reveal. If the fallback rendered,
		* wait out the rest of `pendingMinMs` before replacing it. If it never
		* rendered, there is no minimum wait; if another load took it over, that
		* load owns the deadline.
		*/
		clearTimeout(pending[3]);
		if (pending[4]) {
			const signal = tx[0].signal;
			let rendered = false;
			try {
				rendered = await waitFor(pending[4], signal);
			} catch (cause) {
				if (cause !== signal) throw cause;
			}
			if (rendered && router._pending === pending && pending[0] === tx) {
				const remaining = pending[2] - Date.now();
				if (remaining > 0) {
					try {
						await waitFor(new Promise((resolve) => {
							pending[3] = setTimeout(resolve, remaining);
						}), signal);
					} catch {}
					clearTimeout(pending[3]);
				}
			}
		}
	}
	if (router._tx !== tx) {
		finishPending(router, tx);
		discardLane(router, result);
		return;
	}
	const toLocation = tx[2];
	const changeInfo = require_router.getLocationChangeInfo(toLocation, router.stores.resolvedLocation.get());
	const background = result[2];
	await router.startViewTransition(async () => {
		if (router._tx !== tx) {
			discardLane(router, result);
			return;
		}
		const commit = () => {
			finishPending(router, tx);
			commitMatches(router, tx, result[1], resolvedPrefix);
			if (router._tx !== tx) return;
			router.emit({
				type: "onLoad",
				...changeInfo
			});
			if (router._tx === tx) router.emit({
				type: "onBeforeRouteMount",
				...changeInfo
			});
		};
		const rendered = process.env.NODE_ENV !== "production" && tx[6] ? await transitionRefresh(router, tx, result, changeInfo) : await router.startTransition(commit, result[1]);
		if (process.env.NODE_ENV !== "production" && tx[6] && rendered === void 0) return;
		if (router._tx !== tx) {
			discardBackground(router, result);
			return;
		}
		if (background?.length) runBackground(router, tx, result[1], background, result[3]).catch(console.error);
		router.batch(() => {
			router.stores.resolvedLocation.set(toLocation);
			router.stores.status.set("idle");
			if (router._tx === tx) router.emit({
				type: "onResolved",
				...changeInfo
			});
			if (rendered && router._tx === tx) router.emit({
				type: "onRendered",
				...changeInfo
			});
		});
		if (router._tx !== tx) return;
		router._commitPromise?.resolve();
		router._commitPromise = void 0;
	});
}
async function loadClientRoute(router, opts) {
	let rematerialize = false;
	if (process.env.NODE_ENV !== "production") {
		router._tx?.[6]?.[2]?.();
		rematerialize = !!router._refreshNextLoad || !!router._tx?.[6];
	}
	const refreshPresentation = rematerialize ? router.stores.matches.get() : void 0;
	const previousOwner = router._tx;
	const resolvedLocation = router.stores.resolvedLocation.get();
	const previousLocation = resolvedLocation ?? router.stores.location.get();
	const location = router.latestLocation;
	const pendingLocation = router._pendingLocation;
	const redirects = pendingLocation?.href === location.href ? pendingLocation._redirects ?? 0 : 0;
	const handoff = router._handoff;
	const hydrationController = rematerialize ? void 0 : handoff?.[0]();
	const preflight = new AbortController();
	const previousPreflight = router._preflight;
	router._preflight = preflight;
	if (!rematerialize && !hydrationController) handoff?.[1]();
	previousPreflight?.abort();
	if (preflight.signal.aborted) {
		await awaitCurrent(router, previousOwner);
		return;
	}
	const changeInfo = require_router.getLocationChangeInfo(location, resolvedLocation);
	router.emit({
		type: "onBeforeNavigate",
		...changeInfo
	});
	if (!preflight.signal.aborted) router.emit({
		type: "onBeforeLoad",
		...changeInfo
	});
	if (preflight.signal.aborted) {
		await awaitCurrent(router, previousOwner);
		return;
	}
	const sameHref = previousLocation.href === location.href;
	let matches;
	let controller = preflight;
	try {
		matches = process.env.NODE_ENV !== "production" && rematerialize ? router.matchRoutes(location, {
			_controller: preflight,
			_rematerialize: true
		}) : router.matchRoutes(location, { _controller: preflight });
		acquireMatchResources(matches);
	} catch (cause) {
		preflight.abort();
		if (!require_redirect.isRedirect(cause)) {
			if (process.env.NODE_ENV !== "production" && rematerialize) router._refreshNextLoad = void 0;
			await awaitCurrent(router);
			router._commitPromise?.resolve();
			router._commitPromise = void 0;
			return;
		}
		await router.navigate({
			...cause.options,
			replace: true,
			ignoreBlocker: true
		});
		await awaitCurrent(router, previousOwner);
		return;
	}
	const resolvedPrefix = hydrationController ? handoff[1](matches) : void 0;
	if (resolvedPrefix) controller = hydrationController;
	else hydrationController?.abort();
	if (preflight.signal.aborted) {
		transferMatchResources(router, matches);
		await awaitCurrent(router, previousOwner);
		return;
	}
	router._preflight = void 0;
	const tx = [
		controller,
		redirects,
		location,
		matches,
		Date.now(),
		Promise.resolve().then(() => runClientTransaction(router, tx, sameHref, () => offerPending(router, tx), opts?.sync, resolvedPrefix)).catch(() => {
			if (router._tx === tx) restoreCommitted(router, tx);
		})
	];
	if (process.env.NODE_ENV !== "production" && rematerialize) {
		tx[6] = [refreshPresentation, handoff];
		router._refreshNextLoad = void 0;
	}
	router._tx = tx;
	if (previousOwner) {
		for (const match of router.stores.matches.get()) {
			if (router._tx !== tx) break;
			if (match.isFetching) setFetching(router, match, false);
		}
		previousOwner[0].abort();
		transferMatchResources(router, previousOwner[3], tx[3], true);
	}
	if (router._tx !== tx) {
		transferMatchResources(router, tx[3]);
		tx[3] = [];
		await awaitCurrent(router, tx);
		return;
	}
	router.batch(() => {
		router.stores.status.set("pending");
		router.stores.location.set(location);
	});
	if (!resolvedLocation && !matches.some((match) => match._notFound)) offerPending(router, tx);
	try {
		await tx[5];
	} finally {
		await awaitCurrent(router, tx);
	}
}
async function refreshClientRoute(router) {
	router._tx?.[6]?.[2]?.();
	const pending = router._tx;
	if (pending && !pending[6] && router.stores.status.get() === "pending") {
		await pending[5];
		if (router._tx !== pending) await awaitCurrent(router, pending);
	}
	router._flights?.clear();
	router.clearCache();
	router._refreshNextLoad = true;
	await loadClientRoute(router, { sync: true });
}
async function preloadClientRoute(router, opts, redirects = 0) {
	if (redirects > 20) return;
	if (process.env.NODE_ENV !== "production" && (router._refreshNextLoad || router._tx?.[6])) return;
	const location = opts._builtLocation ?? router.buildLocation(opts);
	const base = router._committed;
	const controller = new AbortController();
	let matches;
	try {
		matches = router.matchRoutes(location, { _controller: controller });
		acquireMatchResources(matches);
	} catch (cause) {
		controller.abort();
		if (!require_not_found.isNotFound(cause)) console.error(cause);
		return;
	}
	(router._preloads ??= /* @__PURE__ */ new Map()).set(controller, matches);
	let active;
	try {
		let result;
		try {
			result = await executeClientLane(router, location, matches, [
				controller,
				redirects,
				() => true,
				base,
				true
			]);
		} finally {
			active = router._preloads.delete(controller);
			transferMatchResources(router, matches);
			controller.abort();
		}
		if (!isControl(result)) return result[1];
		if (active && result[0] === REDIRECTED && !result[1].options.reloadDocument) return preloadClientRoute(router, {
			...result[1].options,
			_fromLocation: location
		}, redirects + 1);
	} catch (cause) {
		if (!require_not_found.isNotFound(cause)) console.error(cause);
	}
}
async function hydrate(router) {
	if (process.env.NODE_ENV !== "production" && !window.$_TSR) throw new Error("Invariant failed: Expected to find bootstrap data on window.$_TSR, but we did not. Please file an issue!");
	const tsr = window.$_TSR;
	const adapters = router.options.serializationAdapters;
	if (adapters?.length) {
		tsr.t = new Map(adapters.map((adapter) => [adapter.key, adapter.fromSerializable]));
		tsr.buffer.forEach((script) => script());
	}
	tsr.initialized = true;
	const dehydratedRouter = tsr.router;
	if (process.env.NODE_ENV !== "production" && !dehydratedRouter) throw new Error("Invariant failed: Expected to find a dehydrated data on window.$_TSR.router, but we did not. Please file an issue!");
	router.ssr = { manifest: dehydratedRouter.manifest };
	router.options.ssr = { nonce: document.querySelector("meta[property=\"csp-nonce\"]")?.content };
	const dehydratedMatches = dehydratedRouter.matches;
	const controller = new AbortController();
	const previousPreflight = router._preflight;
	router._preflight = controller;
	previousPreflight?.abort();
	const isCurrent = () => router._preflight === controller;
	let location;
	let candidates;
	let handoffHistoryHref;
	let handoffHistoryState;
	try {
		await waitFor(router.options.hydrate?.(dehydratedRouter.dehydratedData), controller.signal);
		if (!isCurrent()) return;
		const historyLocation = router.history.location;
		handoffHistoryHref = historyLocation.href;
		handoffHistoryState = historyLocation.state;
		router.updateLatestLocation();
		location = router.latestLocation;
		router.stores.location.set(location);
		candidates = router.matchRoutes(location, { _controller: controller });
	} catch (cause) {
		if (isCurrent()) router._preflight = void 0;
		controller.abort(cause);
		if (cause !== controller.signal) throw cause;
	}
	if (!isCurrent()) return;
	const committed = [];
	let pendingBoundary;
	let verifiedAssetEnd = 0;
	const retryFrom = (index) => {
		verifiedAssetEnd = Math.min(verifiedAssetEnd, index + 1);
		const removed = committed.splice(index);
		for (const match of removed) if (getRoute(router, match).options.loader && (match.status === "success" || !match.invalid && "loaderData" in match)) cacheLoaderMatch(router, {
			...match,
			status: "success",
			error: void 0,
			preload: true
		}, router._cache.get(match.id));
		transferMatchResources(router, removed);
	};
	const shared = dehydratedMatches.length > candidates.length ? candidates.findIndex((match) => match._notFound) + 1 : dehydratedMatches.length;
	let isTerminal = false;
	for (let index = 0; index < shared; index++) {
		const candidate = candidates[index];
		const dehydrated = dehydratedMatches[index];
		if (typeof dehydrated.i !== "string" || require_ssr_match_id.hydrateSsrMatchId(dehydrated.i) !== candidate.id) {
			pendingBoundary ??= index;
			break;
		}
		verifiedAssetEnd = index + 1;
		const route = getRoute(router, candidate);
		if ("l" in dehydrated || dehydrated.s === "success" && dehydrated.e === void 0 && route.options.loader) candidate.loaderData = dehydrated.l;
		candidate.status = dehydrated.s;
		candidate.ssr = dehydrated.ssr;
		route.options.ssr = candidate.ssr;
		candidate.updatedAt = dehydrated.u;
		candidate.error = dehydrated.e;
		candidate._notFound ||= dehydrated.g;
		if (candidate.status === "error" || candidate.status === "notFound" || candidate._notFound) {
			isTerminal = true;
			committed.push(candidate);
			if (candidate.ssr === false || candidate.ssr === "data-only") pendingBoundary ??= index;
			break;
		}
		if (candidate.status === "pending") {
			pendingBoundary ??= index;
			break;
		}
		committed.push(candidate);
		if (candidate.ssr === "data-only") pendingBoundary ??= index;
	}
	if (!isTerminal && committed.length === shared && shared < candidates.length) pendingBoundary = shared;
	const chunks = committed.map(async (match) => {
		try {
			const route = getRoute(router, match);
			if (match._notFound) await Promise.all([loadRouteChunk(route), loadRouteChunk(route, "notFoundComponent")]);
			else await loadRouteChunk(route, match.status === "error" ? "errorComponent" : match.status === "notFound" ? "notFoundComponent" : void 0);
			return true;
		} catch {
			return false;
		}
	});
	let chunkFailure = 0;
	try {
		while (chunkFailure < chunks.length && await waitFor(chunks[chunkFailure], controller.signal)) chunkFailure++;
	} catch {
		return;
	}
	if (!isCurrent()) return;
	if (chunkFailure < committed.length) retryFrom(chunkFailure);
	const contextEnd = Math.max(pendingBoundary === committed.length ? committed.length + 1 : committed.length, chunkFailure < chunks.length ? chunkFailure : verifiedAssetEnd);
	for (let index = 0; index < contextEnd; index++) {
		const match = candidates[index];
		const route = getRoute(router, match);
		const parentContext = candidates[index - 1]?.context ?? router.options.context ?? {};
		let routeContext;
		if (route.options.context) {
			try {
				routeContext = match._ctx = route.options.context({
					deps: match.loaderDeps,
					params: match.params,
					context: parentContext,
					location,
					navigate: navigateFrom(router, location),
					buildLocation: router.buildLocation,
					cause: match.cause,
					abortController: controller,
					preload: false,
					matches: candidates,
					routeId: route.id
				}) || {};
			} catch {
				if (!isCurrent()) return;
				if (match.status !== "error" && match.status !== "notFound" && !match._notFound) {
					retryFrom(index);
					break;
				}
			}
			if (!isCurrent()) return;
		}
		match.context = {
			...parentContext,
			...routeContext,
			...committed[index] && dehydratedMatches[index].b
		};
	}
	await projectLane(router, [location, candidates], controller.signal, 0, verifiedAssetEnd);
	if (!isCurrent()) return;
	const needsClientLoad = pendingBoundary !== void 0 || committed.length < shared;
	const committedMatches = isTerminal && committed.length === shared ? candidates : committed;
	let presented = needsClientLoad ? candidates : committedMatches;
	let dataOnlyAssetEnd;
	if (needsClientLoad && pendingBoundary !== void 0) {
		const boundary = presented[pendingBoundary];
		dataOnlyAssetEnd = boundary.ssr === "data-only" && verifiedAssetEnd > pendingBoundary + 1 ? verifiedAssetEnd : void 0;
		presented = presented.slice();
		presented[pendingBoundary] = {
			...boundary,
			status: "pending",
			ssr: boundary.ssr === "data-only" ? "data-only" : false,
			_assetEnd: dataOnlyAssetEnd
		};
	}
	const claim = () => {
		const historyLocation = router.history.location;
		return needsClientLoad && !router._tx && historyLocation.href === handoffHistoryHref && historyLocation.state === handoffHistoryState && router._committed === committedMatches && committedMatches.length && !controller.signal.aborted ? controller : void 0;
	};
	const handoff = [claim, (matches) => {
		if (router._handoff !== handoff) return;
		router._handoff = void 0;
		const prefix = committedMatches.length;
		if (!matches || !claim() || committedMatches.some((match, index) => match.id !== matches[index]?.id)) {
			controller.abort();
			return;
		}
		let handoffAssetEnd = dataOnlyAssetEnd;
		if (handoffAssetEnd !== void 0) {
			for (let index = prefix; index < handoffAssetEnd; index++) if (candidates[index]?.id !== matches[index]?.id) {
				handoffAssetEnd = index > pendingBoundary + 1 ? index : void 0;
				break;
			}
		}
		const clones = committedMatches.map((match) => ({ ...match }));
		if (handoffAssetEnd !== void 0) clones[pendingBoundary]._assetEnd = handoffAssetEnd;
		transferMatchResources(router, matches.splice(0, prefix, ...clones));
		for (let index = prefix; index < matches.length; index++) {
			const match = matches[index];
			const hydrated = candidates[index];
			if (hydrated?.id === match.id && hydrated._ctx) match._ctx = hydrated._ctx;
			match.abortController = controller;
		}
		return prefix;
	}];
	router._committed = committedMatches;
	router._handoff = handoff;
	router._preflight = void 0;
	router.batch(() => {
		router.stores.setMatches(presented);
		router.stores.status.set("idle");
		if (!needsClientLoad) router.stores.resolvedLocation.set(router.stores.location.get());
	});
}
//#endregion
exports._getAssetMatches = _getAssetMatches;
exports._getRenderedMatches = _getRenderedMatches;
exports.hydrate = hydrate;
exports.loadClientRoute = loadClientRoute;
exports.loadRouteChunk = loadRouteChunk;
exports.preloadClientRoute = preloadClientRoute;
exports.refreshClientRoute = refreshClientRoute;
exports.replaceRouteChunk = replaceRouteChunk;

//# sourceMappingURL=load-client.cjs.map