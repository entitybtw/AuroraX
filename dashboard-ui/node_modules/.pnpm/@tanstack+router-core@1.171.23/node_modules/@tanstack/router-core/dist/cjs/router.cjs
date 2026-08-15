const require_utils = require("./utils.cjs");
const require_lru_cache = require("./lru-cache.cjs");
const require_new_process_route_tree = require("./new-process-route-tree.cjs");
const require_path = require("./path.cjs");
const require_not_found = require("./not-found.cjs");
const require_scroll_restoration = require("./scroll-restoration.cjs");
const require_searchParams = require("./searchParams.cjs");
const require_root = require("./root.cjs");
const require_redirect = require("./redirect.cjs");
const require_load_client = require("./load-client.cjs");
const require_rewrite = require("./rewrite.cjs");
const require_stores = require("./stores.cjs");
let _tanstack_history = require("@tanstack/history");
let _tanstack_router_core_isServer = require("@tanstack/router-core/isServer");
//#region src/router.ts
function routeNeedsLoad(route) {
	return route.options.loader || route.options.beforeLoad || route.lazyFn || route.options.component?.preload || route.options.pendingComponent?.preload;
}
/**
* Convert an unknown error into a minimal, serializable object.
* Includes name and message (and stack in development).
*/
function defaultSerializeError(err) {
	if (err instanceof Error) {
		const obj = {
			name: err.name,
			message: err.message
		};
		if (process.env.NODE_ENV === "development") obj.stack = err.stack;
		return obj;
	}
	return { data: err };
}
/** Options for configuring trailing-slash behavior. */
const trailingSlashOptions = {
	always: "always",
	never: "never",
	preserve: "preserve"
};
/**
* Compute whether path, href or hash changed between previous and current
* resolved locations.
*/
function getLocationChangeInfo(location, resolvedLocation) {
	return {
		fromLocation: resolvedLocation,
		toLocation: location,
		pathChanged: resolvedLocation?.pathname !== location.pathname,
		hrefChanged: resolvedLocation?.href !== location.href,
		hashChanged: resolvedLocation?.hash !== location.hash
	};
}
/**
* Return only state owned by the application, excluding volatile history
* bookkeeping. Mask payloads (`__tempLocation`/`__tempKey`) are kept: they
* distinguish otherwise-identical locations.
*/
function _getUserHistoryState({ key: _key, __TSR_key: _tsrKey, __TSR_index: _tsrIndex, __hashScrollIntoViewOptions: _hashScroll, ...state }) {
	return state;
}
/** Run route lifecycle callbacks in leave/enter/stay phases. */
function runRouteLifecycle(router, previous, matches, isCurrent) {
	for (const match of previous) {
		if (isCurrent?.() === false) return;
		if (!matches.some((candidate) => candidate.routeId === match.routeId)) router.routesById[match.routeId].options.onLeave?.(match);
	}
	for (const match of matches) {
		if (isCurrent?.() === false) return;
		router.routesById[match.routeId].options[previous.some((candidate) => candidate.routeId === match.routeId) ? "onStay" : "onEnter"]?.(match);
	}
}
/**
* Core, framework-agnostic router engine that powers TanStack Router.
*
* Provides navigation, matching, loading, preloading, caching and event APIs
* used by framework adapters (React/Solid). Prefer framework helpers like
* `createRouter` in app code.
*
* @link https://tanstack.com/router/latest/docs/framework/react/api/router/RouterType
*/
var RouterCore = class {
	/**
	* @deprecated Use the `createRouter` function instead
	*/
	constructor(options, getStoreConfig) {
		this.tempLocationKey = `${Math.round(Math.random() * 1e7)}`;
		this._scroll = { next: true };
		this.subscribers = /* @__PURE__ */ new Set();
		this._cache = /* @__PURE__ */ new Map();
		this._committed = [];
		this.routeBranchCache = /* @__PURE__ */ new WeakMap();
		this.lightweightCache = /* @__PURE__ */ new WeakMap();
		this.startTransition = async (fn) => {
			fn();
			return false;
		};
		this.update = (newOptions) => {
			if (process.env.NODE_ENV !== "production") {
				if (newOptions.notFoundRoute) console.warn("The notFoundRoute API is deprecated and will be removed in the next major version. See https://tanstack.com/router/v1/docs/framework/react/guide/not-found-errors#migrating-from-notfoundroute for more info.");
			}
			const prevOptions = this.options;
			const prevBasepath = this.basepath ?? prevOptions?.basepath ?? "/";
			const basepathWasUnset = this.basepath === void 0;
			const prevRewriteOption = prevOptions?.rewrite;
			this.options = {
				...prevOptions,
				...newOptions
			};
			this.isServer = this.options.isServer ?? _tanstack_router_core_isServer.isServer ?? typeof document === "undefined";
			this.protocolAllowlist = new Set(this.options.protocolAllowlist);
			if (this.options.pathParamsAllowedCharacters) this.pathParamsDecoder = require_path.compileDecodeCharMap(this.options.pathParamsAllowedCharacters);
			if (!this.history || this.options.history && this.options.history !== this.history) if (!this.options.history) {
				if (!(_tanstack_router_core_isServer.isServer ?? this.isServer)) this.history = (0, _tanstack_history.createBrowserHistory)();
			} else this.history = this.options.history;
			this.origin = this.options.origin;
			if (!this.origin) if (!(_tanstack_router_core_isServer.isServer ?? this.isServer) && window?.origin && window.origin !== "null") this.origin = window.origin;
			else this.origin = "http://localhost";
			if (this.history) this.updateLatestLocation();
			if (this.options.routeTree !== this.routeTree) {
				this.routeTree = this.options.routeTree;
				let processRouteTreeResult;
				if ((_tanstack_router_core_isServer.isServer ?? this.isServer) && process.env.NODE_ENV !== "development" && globalThis.__TSR_CACHE__ && globalThis.__TSR_CACHE__.routeTree === this.routeTree) {
					const cached = globalThis.__TSR_CACHE__;
					this.resolvePathCache = cached.resolvePathCache;
					processRouteTreeResult = cached.processRouteTreeResult;
				} else {
					this.resolvePathCache = require_lru_cache.createLRUCache(1e3);
					processRouteTreeResult = this.buildRouteTree();
					if ((_tanstack_router_core_isServer.isServer ?? this.isServer) && process.env.NODE_ENV !== "development" && globalThis.__TSR_CACHE__ === void 0) globalThis.__TSR_CACHE__ = {
						routeTree: this.routeTree,
						processRouteTreeResult,
						resolvePathCache: this.resolvePathCache
					};
				}
				this.setRoutes(processRouteTreeResult);
			}
			if (!this.stores && this.latestLocation) {
				const config = this.getStoreConfig(this);
				this.batch = config.batch;
				this.stores = require_stores.createRouterStores(this.latestLocation, config);
				if (!(_tanstack_router_core_isServer.isServer ?? this.isServer)) require_scroll_restoration.setupScrollRestoration(this);
			}
			const nextBasepath = this.options.basepath ?? "/";
			const nextRewriteOption = this.options.rewrite;
			if (basepathWasUnset || prevBasepath !== nextBasepath || prevRewriteOption !== nextRewriteOption) {
				this.basepath = nextBasepath;
				const rewrites = [];
				const trimmed = require_path.trimPath(nextBasepath);
				if (trimmed && trimmed !== "/") rewrites.push(require_rewrite.rewriteBasepath({ basepath: nextBasepath }));
				if (nextRewriteOption) rewrites.push(nextRewriteOption);
				this.rewrite = rewrites.length === 0 ? void 0 : rewrites.length === 1 ? rewrites[0] : require_rewrite.composeRewrites(rewrites);
				if (this.history) this.updateLatestLocation();
				if (this.stores) this.stores.location.set(this.latestLocation);
			}
		};
		this.updateLatestLocation = () => {
			this.latestLocation = this.parseLocation(this.history.location, this.latestLocation);
		};
		this.buildRouteTree = () => {
			const result = require_new_process_route_tree.processRouteTree(this.routeTree, this.options.caseSensitive, (route, i) => {
				route.init({ originalIndex: i });
			});
			if (this.options.routeMasks) require_new_process_route_tree.processRouteMasks(this.options.routeMasks, result.processedTree);
			return result;
		};
		this.subscribe = (eventType, fn) => {
			const listener = {
				eventType,
				fn
			};
			this.subscribers.add(listener);
			return () => {
				this.subscribers.delete(listener);
			};
		};
		this.emit = (routerEvent) => {
			for (const listener of this.subscribers) if (listener.eventType === routerEvent.type) try {
				listener.fn(routerEvent);
			} catch (e) {
				console.error(e);
			}
		};
		this.parseLocation = (locationToParse, previousLocation) => {
			const parse = ({ pathname, search, hash, href, state }) => {
				if (!this.rewrite && !/[ \x00-\x1f\x7f\u0080-\uffff]/.test(pathname)) {
					const parsedSearch = this.options.parseSearch(search);
					const searchStr = this.options.stringifySearch(parsedSearch);
					return {
						href: pathname + searchStr + hash,
						publicHref: pathname + searchStr + hash,
						pathname: require_utils.decodePath(pathname).path,
						external: false,
						searchStr,
						search: require_utils.nullReplaceEqualDeep(previousLocation?.search, parsedSearch),
						hash: require_utils.decodePath(hash.slice(1)).path,
						state: require_utils.replaceEqualDeep(previousLocation?.state, state)
					};
				}
				const fullUrl = new URL(href, this.origin);
				const url = require_rewrite.executeRewriteInput(this.rewrite, fullUrl);
				const parsedSearch = this.options.parseSearch(url.search);
				const searchStr = this.options.stringifySearch(parsedSearch);
				url.search = searchStr;
				return {
					href: url.href.replace(url.origin, ""),
					publicHref: href,
					pathname: require_utils.decodePath(url.pathname).path,
					external: !!this.rewrite && url.origin !== this.origin,
					searchStr,
					search: require_utils.nullReplaceEqualDeep(previousLocation?.search, parsedSearch),
					hash: require_utils.decodePath(url.hash.slice(1)).path,
					state: require_utils.replaceEqualDeep(previousLocation?.state, state)
				};
			};
			const location = parse(locationToParse);
			const { __tempLocation, __tempKey } = location.state;
			if (__tempLocation && (!__tempKey || __tempKey === this.tempLocationKey)) {
				const parsedTempLocation = parse(__tempLocation);
				parsedTempLocation.state.key = location.state.key;
				parsedTempLocation.state.__TSR_key = location.state.__TSR_key;
				delete parsedTempLocation.state.__tempLocation;
				return {
					...parsedTempLocation,
					maskedLocation: location
				};
			}
			return location;
		};
		this.resolvePathWithBase = (from, path) => {
			return require_path.resolvePath({
				base: from,
				to: path.includes("//") ? require_path.cleanPath(path) : path,
				trailingSlash: this.options.trailingSlash,
				cache: this.resolvePathCache
			});
		};
		this.matchRoutes = (pathnameOrNext, locationSearchOrOpts, opts) => {
			if (typeof pathnameOrNext === "string") return this.matchRoutesInternal({
				pathname: pathnameOrNext,
				search: locationSearchOrOpts
			}, opts);
			return this.matchRoutesInternal(pathnameOrNext, locationSearchOrOpts);
		};
		this.getMatchedRoutes = (pathname) => {
			const rawParams = Object.create(null);
			const match = require_new_process_route_tree.findRouteMatch(require_path.trimPathRight(pathname), this.processedTree, true);
			if (match) Object.assign(rawParams, match.rawParams);
			return [
				match?.branch || [this.routesById["__root__"]],
				rawParams,
				match?.route
			];
		};
		this.buildLocation = (opts) => {
			const build = (dest = {}) => {
				const currentLocation = dest._fromLocation || this._pendingLocation || this.latestLocation;
				const lightweightResult = this.matchRoutesLightweight(currentLocation);
				if (dest.from && process.env.NODE_ENV !== "production" && dest._isNavigate) {
					const [allFromMatches] = this.getMatchedRoutes(dest.from);
					const matchedFrom = require_utils.findLast(lightweightResult[0], (d) => {
						return comparePaths(d.fullPath, dest.from);
					});
					const matchedCurrent = require_utils.findLast(allFromMatches, (d) => {
						return comparePaths(d.fullPath, lightweightResult[1]);
					});
					if (!matchedFrom && !matchedCurrent) console.warn(`Could not find match for from: ${dest.from}`);
				}
				const defaultedFromPath = dest.unsafeRelative === "path" ? currentLocation.pathname : dest.from ?? lightweightResult[1];
				const destTo = dest.to ? `${dest.to}` : void 0;
				const fromSearch = lightweightResult[2];
				const fromParams = Object.assign(Object.create(null), lightweightResult[3]);
				const sourcePath = destTo?.charCodeAt(0) === 47 ? "/" : this.resolvePathWithBase(defaultedFromPath, ".");
				const nextTo = destTo ? this.resolvePathWithBase(sourcePath, destTo) : sourcePath;
				const nextParams = resolveNextParams(dest.params, fromParams);
				const destRoute = this.routesByPath[require_path.trimPathRight(nextTo)];
				let destRoutes;
				if (destRoute) destRoutes = this.getRouteBranch(destRoute);
				else if (nextTo.includes("$")) destRoutes = [];
				else {
					const [matchedRoutes, rawParams, foundRoute] = this.getMatchedRoutes(nextTo);
					destRoutes = matchedRoutes;
					if (this.options.notFoundRoute && (!foundRoute || foundRoute.path !== "/" && rawParams["**"])) destRoutes = [...destRoutes, this.options.notFoundRoute];
				}
				if (destRoutes.length && require_utils.hasKeys(nextParams)) for (const route of destRoutes) {
					const fn = route.options.params?.stringify ?? route.options.stringifyParams;
					if (fn) try {
						Object.assign(nextParams, fn(nextParams));
					} catch {}
				}
				const nextPathname = opts.leaveParams ? nextTo : require_utils.decodePath(require_path.interpolatePath({
					path: nextTo,
					params: nextParams,
					decoder: this.pathParamsDecoder,
					server: this.isServer
				}).interpolatedPath).path;
				if (process.env.NODE_ENV !== "production" && destRoute && !opts.leaveParams) try {
					const foundRoute = this.getMatchedRoutes(nextPathname)[2];
					if (foundRoute?.id !== destRoute.id) console.warn(`Generated path "${nextPathname}" for route "${destRoute.id}" matched route "${foundRoute?.id}" instead. This can happen when multiple route templates resolve to the same URL. Use the route template that matches the intended route, or adjust params.stringify if it changed the target path.`);
				} catch {}
				let nextSearch = fromSearch;
				if (opts._includeValidateSearch && this.options.search?.strict) {
					const validatedSearch = {};
					destRoutes.forEach((route) => {
						if (route.options.validateSearch) try {
							Object.assign(validatedSearch, validateSearch(route.options.validateSearch, {
								...validatedSearch,
								...nextSearch
							}));
						} catch {}
					});
					nextSearch = validatedSearch;
				}
				nextSearch = applySearchMiddleware(nextSearch, dest, destRoutes, opts._includeValidateSearch);
				nextSearch = require_utils.nullReplaceEqualDeep(fromSearch, nextSearch);
				const searchStr = this.options.stringifySearch(nextSearch);
				const hash = dest.hash === true ? currentLocation.hash : dest.hash ? require_utils.functionalUpdate(dest.hash, currentLocation.hash) : void 0;
				const hashStr = hash ? `#${hash}` : "";
				let nextState = dest.state === true ? currentLocation.state : dest.state ? require_utils.functionalUpdate(dest.state, currentLocation.state) : {};
				nextState = require_utils.replaceEqualDeep(currentLocation.state, nextState);
				const fullPath = `${nextPathname}${searchStr}${hashStr}`;
				let href;
				let publicHref;
				let external = false;
				if (this.rewrite) {
					const url = new URL(fullPath, this.origin);
					const rewrittenUrl = require_rewrite.executeRewriteOutput(this.rewrite, url);
					href = url.href.replace(url.origin, "");
					if (rewrittenUrl.origin !== this.origin) {
						publicHref = rewrittenUrl.href;
						external = true;
					} else publicHref = rewrittenUrl.pathname + rewrittenUrl.search + rewrittenUrl.hash;
				} else {
					href = require_utils.encodePathLikeUrl(fullPath);
					publicHref = href;
				}
				return {
					publicHref,
					href,
					pathname: nextPathname,
					search: nextSearch,
					searchStr,
					state: nextState,
					hash: hash ?? "",
					external,
					unmaskOnReload: dest.unmaskOnReload
				};
			};
			const buildWithMatches = (dest = {}, maskedDest) => {
				const next = build(dest);
				let maskedNext = maskedDest ? build(maskedDest) : void 0;
				if (!maskedNext) {
					const params = Object.create(null);
					if (this.options.routeMasks) {
						const match = require_new_process_route_tree.findFlatMatch(next.pathname, this.processedTree);
						if (match) {
							Object.assign(params, match.rawParams);
							const { from: _from, params: maskParams, ...maskProps } = match.route;
							const nextParams = resolveNextParams(maskParams, params);
							maskedDest = {
								from: opts.from,
								...maskProps,
								params: nextParams
							};
							maskedNext = build(maskedDest);
						}
					}
				}
				if (maskedNext) next.maskedLocation = maskedNext;
				return next;
			};
			if (opts.mask) return buildWithMatches(opts, {
				from: opts.from,
				...opts.mask
			});
			return buildWithMatches(opts);
		};
		this.commitLocation = async ({ viewTransition, ignoreBlocker, ...next }) => {
			let historyAction;
			const isSameLocation = require_path.trimPathRight(this.latestLocation.href) === require_path.trimPathRight(next.href) && require_utils.deepEqual(_getUserHistoryState(next.state), _getUserHistoryState(this.latestLocation.state));
			const previousCommitPromise = this._commitPromise;
			let resolve;
			const commitPromise = new Promise((done) => {
				resolve = done;
			});
			commitPromise.resolve = () => {
				resolve();
				previousCommitPromise?.resolve();
			};
			this._commitPromise = commitPromise;
			if (isSameLocation) this.load();
			else {
				let { maskedLocation, hashScrollIntoView, ...nextHistory } = next;
				if (maskedLocation) {
					nextHistory = {
						...maskedLocation,
						state: {
							...maskedLocation.state,
							__tempKey: void 0,
							__tempLocation: {
								...nextHistory,
								search: nextHistory.searchStr,
								state: {
									...nextHistory.state,
									__tempKey: void 0,
									__tempLocation: void 0,
									__TSR_key: void 0,
									key: void 0
								}
							}
						}
					};
					if (nextHistory.unmaskOnReload ?? this.options.unmaskOnReload ?? false) nextHistory.state.__tempKey = this.tempLocationKey;
				}
				nextHistory.state.__hashScrollIntoViewOptions = hashScrollIntoView ?? this.options.defaultHashScrollIntoView ?? true;
				this.shouldViewTransition = viewTransition;
				historyAction = next.replace ? "REPLACE" : "PUSH";
				this.history[historyAction === "REPLACE" ? "replace" : "push"](nextHistory.publicHref, nextHistory.state, { ignoreBlocker });
				if (!this.history.subscribers.size) this.load({ action: { type: historyAction } });
			}
			this._scroll.next = next.resetScroll ?? true;
			return this._commitPromise;
		};
		this.buildAndCommitLocation = ({ replace, resetScroll, hashScrollIntoView, viewTransition, ignoreBlocker, _redirects, href, ...rest } = {}) => {
			if (href) {
				const currentIndex = this.history.location.state.__TSR_index;
				const parsed = (0, _tanstack_history.parseHref)(href, { __TSR_index: replace ? currentIndex : currentIndex + 1 });
				const hrefUrl = new URL(parsed.pathname, this.origin);
				rest.to = require_rewrite.executeRewriteInput(this.rewrite, hrefUrl).pathname;
				rest.search = this.options.parseSearch(parsed.search);
				rest.hash = parsed.hash.slice(1);
			}
			const location = this.buildLocation({
				...rest,
				_includeValidateSearch: true
			});
			if (_redirects) location._redirects = _redirects;
			this._pendingLocation = location;
			const commitPromise = this.commitLocation({
				...location,
				viewTransition,
				replace,
				resetScroll,
				hashScrollIntoView,
				ignoreBlocker
			});
			queueMicrotask(() => {
				if (this._pendingLocation === location) this._pendingLocation = void 0;
			});
			return commitPromise;
		};
		this.navigate = async ({ to, reloadDocument, href, publicHref, ...rest }) => {
			let hrefIsUrl = false;
			if (href) try {
				new URL(`${href}`);
				hrefIsUrl = true;
			} catch {}
			if (hrefIsUrl && !reloadDocument) reloadDocument = true;
			if (reloadDocument) {
				if (to !== void 0 || !href) {
					const location = this.buildLocation({
						to,
						...rest
					});
					href = href ?? location.publicHref;
					publicHref = publicHref ?? location.publicHref;
				}
				const reloadHref = !hrefIsUrl && publicHref ? publicHref : href;
				if (require_utils.isDangerousProtocol(reloadHref, this.protocolAllowlist)) {
					if (process.env.NODE_ENV !== "production") console.warn(`Blocked navigation to dangerous protocol: ${reloadHref}`);
					return;
				}
				if (!rest.ignoreBlocker) {
					const blockers = this.history.getBlockers?.() ?? [];
					for (const blocker of blockers) if (blocker?.blockerFn) {
						if (await blocker.blockerFn({
							currentLocation: this.latestLocation,
							nextLocation: this.latestLocation,
							action: "PUSH"
						})) return;
					}
				}
				if (rest.replace) window.location.replace(reloadHref);
				else window.location.href = reloadHref;
				return;
			}
			return this.buildAndCommitLocation({
				...rest,
				href,
				to,
				_isNavigate: true
			});
		};
		this.load = async (opts) => {
			if (_tanstack_router_core_isServer.isServer ?? this.isServer) return (0, _tanstack_router_core_isServer.loadServerRoute)(this, opts);
			this.updateLatestLocation();
			if (opts?.action) this._scroll.hash = opts.action.type === "PUSH" || opts.action.type === "REPLACE";
			await require_load_client.loadClientRoute(this, opts);
		};
		this.startViewTransition = (fn) => {
			const shouldViewTransition = this.shouldViewTransition ?? this.options.defaultViewTransition;
			this.shouldViewTransition = void 0;
			if (shouldViewTransition && !(_tanstack_router_core_isServer.isServer ?? typeof document === "undefined") && typeof document.startViewTransition === "function") {
				let startViewTransitionParams;
				if (typeof shouldViewTransition === "object" && window.CSS?.supports?.("selector(:active-view-transition-type(a))")) {
					const next = this.latestLocation;
					const prevLocation = this.stores.resolvedLocation.get();
					const resolvedViewTransitionTypes = typeof shouldViewTransition.types === "function" ? shouldViewTransition.types(getLocationChangeInfo(next, prevLocation)) : shouldViewTransition.types;
					if (resolvedViewTransitionTypes === false) return fn();
					startViewTransitionParams = {
						update: fn,
						types: resolvedViewTransitionTypes
					};
				} else startViewTransitionParams = fn;
				return document.startViewTransition(startViewTransitionParams).updateCallbackDone;
			}
			return fn();
		};
		this.invalidate = (opts) => {
			const committedMatches = this._committed;
			const filter = opts?.filter;
			const preloads = this._preloads;
			const invalidIds = new Set([
				...committedMatches,
				...this._cache.values(),
				...[...preloads?.values() ?? []].flat(),
				...this._tx?.[3] ?? []
			].filter((match) => !filter || filter(match)).map((match) => match.id));
			const discardedPreloads = [];
			for (const [controller, matches] of preloads ?? []) if (matches.some((match) => invalidIds.has(match.id))) {
				preloads.delete(controller);
				discardedPreloads.push(controller);
			}
			const invalidate = (d) => {
				if (invalidIds.has(d.id)) {
					const route = this.routesById[d.routeId];
					const next = {
						...d,
						invalid: true,
						...(opts?.forcePending || d.status === "error" || d.status === "notFound") && routeNeedsLoad(route) ? {
							status: "pending",
							error: void 0
						} : void 0
					};
					d._flight = void 0;
					return next;
				}
				return d;
			};
			this._committed = committedMatches.map(invalidate);
			for (const [id, match] of this._cache) if (invalidIds.has(id)) {
				match.invalid = true;
				if (opts?.forcePending) match.status = "pending";
			}
			for (const id of invalidIds) this._flights?.delete(id);
			for (const controller of discardedPreloads) controller.abort();
			this.shouldViewTransition = false;
			return this.load({ sync: opts?.sync });
		};
		this.resolveRedirect = (redirect) => {
			const locationHeader = redirect.headers.get("Location");
			if (!redirect.options.href || redirect.options._builtLocation) {
				const href = (redirect.options._builtLocation ?? this.buildLocation(redirect.options)).publicHref || "/";
				redirect.options.href = href;
				redirect.headers.set("Location", href);
			} else if (locationHeader) try {
				const url = new URL(locationHeader);
				if (this.origin && url.origin === this.origin) {
					const href = url.pathname + url.search + url.hash;
					redirect.options.href = href;
					redirect.headers.set("Location", href);
				}
			} catch {}
			if (redirect.options.href && !redirect.options._builtLocation && require_utils.isDangerousProtocol(redirect.options.href, this.protocolAllowlist)) throw new Error(process.env.NODE_ENV !== "production" ? `Redirect blocked: unsafe protocol in href "${redirect.options.href}". Allowed protocols: ${Array.from(this.protocolAllowlist).join(", ")}.` : "Redirect blocked: unsafe protocol");
			if (!redirect.headers.get("Location")) redirect.headers.set("Location", redirect.options.href);
			return redirect;
		};
		this.clearCache = (opts) => {
			const cached = this._cache;
			const preloads = this._preloads;
			const filter = opts?.filter;
			const discarded = [];
			const discardedIds = [];
			for (const [id, match] of cached) if (!filter || filter(match)) {
				discardedIds.push(id);
				discarded.push(match);
			}
			const abort = [];
			for (const [controller, matches] of preloads ?? []) if (!filter || matches.some(filter)) {
				abort.push(controller);
				discarded.push(...matches);
			}
			for (const id of discardedIds) cached.delete(id);
			for (const controller of abort) preloads.delete(controller);
			for (const match of discarded) {
				const flight = match._flight;
				match._flight = void 0;
				if (flight && !--flight[2]) {
					if (this._flights?.get(match.id) === flight) this._flights.delete(match.id);
					abort.push(flight[1]);
				}
			}
			for (const controller of abort) controller.abort();
		};
		this.loadRouteChunk = require_load_client.loadRouteChunk;
		this.preloadRoute = (opts) => require_load_client.preloadClientRoute(this, opts);
		this.matchRoute = (location, opts) => {
			const matchLocation = {
				...location,
				to: location.to ? this.resolvePathWithBase(location.from || "", location.to) : void 0,
				params: location.params || {},
				leaveParams: true
			};
			const next = this.buildLocation(matchLocation);
			const isPending = this.stores.status.get() === "pending";
			if (opts?.pending && !isPending) return false;
			const baseLocation = opts?.pending ?? !isPending ? this.latestLocation : this.stores.resolvedLocation.get() || this.stores.location.get();
			const match = require_new_process_route_tree.findSingleMatch(next.pathname, opts?.caseSensitive ?? false, opts?.fuzzy ?? false, baseLocation.pathname, this.processedTree);
			if (!match) return false;
			if (location.params) {
				if (!require_utils.deepEqual(match.rawParams, location.params, { partial: true })) return false;
			}
			if (opts?.includeSearch ?? true) return require_utils.deepEqual(baseLocation.search, next.search, { partial: true }) ? match.rawParams : false;
			return match.rawParams;
		};
		this.getStoreConfig = getStoreConfig;
		this.update({
			defaultPreloadDelay: 50,
			defaultPendingMs: 1e3,
			defaultPendingMinMs: 500,
			context: void 0,
			...options,
			caseSensitive: options.caseSensitive ?? false,
			notFoundMode: options.notFoundMode ?? "fuzzy",
			stringifySearch: options.stringifySearch ?? require_searchParams.defaultStringifySearch,
			parseSearch: options.parseSearch ?? require_searchParams.defaultParseSearch,
			protocolAllowlist: options.protocolAllowlist ?? require_utils.DEFAULT_PROTOCOL_ALLOWLIST
		});
		if (!(_tanstack_router_core_isServer.isServer ?? typeof document === "undefined")) self.__TSR_ROUTER__ = this;
	}
	isShell() {
		return !!this.options.isShell;
	}
	get state() {
		return this.stores.__store.get();
	}
	setRoutes({ routesById, routesByPath, processedTree }) {
		this.routesById = routesById;
		this.routesByPath = routesByPath;
		this.processedTree = processedTree;
		const notFoundRoute = this.options.notFoundRoute;
		if (notFoundRoute) {
			notFoundRoute.init({ originalIndex: 99999999999 });
			this.routesById[notFoundRoute.id] = notFoundRoute;
		}
	}
	getRouteBranch(route) {
		let branch = this.routeBranchCache.get(route);
		if (!branch) {
			branch = require_new_process_route_tree.buildRouteBranch(route);
			this.routeBranchCache.set(route, branch);
		}
		return branch;
	}
	matchRoutesInternal(next, opts) {
		const [initialMatchedRoutes, rawParams, foundRoute] = this.getMatchedRoutes(next.pathname);
		let matchedRoutes = initialMatchedRoutes;
		let isGlobalNotFound = false;
		if (foundRoute ? foundRoute.path !== "/" && rawParams["**"] : require_path.trimPathRight(next.pathname)) if (this.options.notFoundRoute) matchedRoutes = [...matchedRoutes, this.options.notFoundRoute];
		else isGlobalNotFound = true;
		const _notFoundRouteId = isGlobalNotFound ? findGlobalNotFoundRouteId(this.options.notFoundMode, matchedRoutes) : void 0;
		const matches = new Array(matchedRoutes.length);
		const committed = this._committed;
		const previousAt = (route, index) => {
			const match = committed[index];
			return match?.routeId === route.id ? match : route === this.options.notFoundRoute ? committed.find((candidate) => candidate.routeId === route.id) : void 0;
		};
		let strictParams;
		for (let index = 0; index < matchedRoutes.length; index++) {
			const route = matchedRoutes[index];
			const parentMatch = matches[index - 1];
			let preMatchSearch;
			let strictMatchSearch;
			let searchError;
			{
				const parentSearch = parentMatch?.search ?? next.search;
				const parentStrictSearch = parentMatch?._strictSearch ?? void 0;
				try {
					const strictSearch = validateSearch(route.options.validateSearch, { ...parentSearch }) ?? void 0;
					preMatchSearch = {
						...parentSearch,
						...strictSearch
					};
					strictMatchSearch = {
						...parentStrictSearch,
						...strictSearch
					};
				} catch (err) {
					let searchParamError = err;
					if (!(err instanceof SearchParamError)) searchParamError = new SearchParamError(err.message, { cause: err });
					if (opts?.throwOnError) throw searchParamError;
					preMatchSearch = parentSearch;
					strictMatchSearch = {};
					searchError = searchParamError;
				}
			}
			let loaderDeps = "";
			let loaderDepsHash = "";
			try {
				loaderDeps = route.options.loaderDeps?.({ search: preMatchSearch }) ?? "";
				loaderDepsHash = loaderDeps ? JSON.stringify(loaderDeps) || "" : "";
			} catch (cause) {
				if (opts?.throwOnError) throw cause;
				searchError ??= cause;
			}
			const { interpolatedPath, usedParams } = require_path.interpolatePath({
				path: route.fullPath,
				params: rawParams,
				decoder: this.pathParamsDecoder,
				server: this.isServer
			});
			const matchId = route.id + interpolatedPath + loaderDepsHash;
			const previousMatch = previousAt(route, index);
			const existingMatch = process.env.NODE_ENV !== "production" && opts?._rematerialize ? void 0 : this._cache.get(matchId) ?? (previousMatch?.id === matchId ? previousMatch : void 0);
			strictParams = existingMatch?._strictParams ?? Object.assign(usedParams, strictParams);
			let paramsError;
			if (!existingMatch) try {
				extractStrictParams(route, strictParams);
			} catch (err) {
				if (require_not_found.isNotFound(err) || require_redirect.isRedirect(err)) paramsError = err;
				else paramsError = new PathParamError(err.message, { cause: err });
				if (opts?.throwOnError) throw paramsError;
			}
			const cause = previousMatch ? "stay" : "enter";
			let match;
			if (existingMatch) match = {
				...existingMatch,
				cause,
				search: previousMatch ? require_utils.nullReplaceEqualDeep(previousMatch.search, preMatchSearch) : require_utils.nullReplaceEqualDeep(existingMatch.search, preMatchSearch),
				_strictSearch: strictMatchSearch,
				searchError
			};
			else {
				const status = routeNeedsLoad(route) ? "pending" : "success";
				match = {
					id: matchId,
					ssr: _tanstack_router_core_isServer.isServer ?? this.isServer ? void 0 : route.options.ssr,
					index,
					routeId: route.id,
					params: previousMatch?.params ?? strictParams,
					_strictParams: strictParams,
					pathname: interpolatedPath,
					updatedAt: Date.now(),
					search: previousMatch ? require_utils.nullReplaceEqualDeep(previousMatch.search, preMatchSearch) : preMatchSearch,
					_strictSearch: strictMatchSearch,
					searchError,
					status,
					isFetching: false,
					error: void 0,
					paramsError,
					context: {},
					abortController: opts?._controller ?? new AbortController(),
					cause,
					loaderDeps: previousMatch ? require_utils.replaceEqualDeep(previousMatch.loaderDeps, loaderDeps) : loaderDeps,
					invalid: false,
					preload: false,
					staticData: route.options.staticData || {},
					fullPath: route.fullPath
				};
			}
			const _notFound = _notFoundRouteId === route.id;
			if (match._notFound && !_notFound) match.error = void 0;
			match._notFound = _notFound;
			matches[index] = match;
		}
		for (let index = 0; index < matches.length; index++) {
			const match = matches[index];
			match.params = match.cause === "stay" ? require_utils.nullReplaceEqualDeep(match.params, strictParams) : strictParams;
			if (opts?._controller) match.context = {};
		}
		return matches;
	}
	/**
	* Lightweight route matching for buildLocation.
	* Only computes fullPath, accumulated search, and params - skipping expensive
	* operations like AbortController, loaderDeps, and full match objects.
	*/
	matchRoutesLightweight(location) {
		const lastRouteId = require_utils.last(this.stores.ids.get());
		const lastStateMatch = lastRouteId ? this.stores.byRoute.get(lastRouteId).get() : void 0;
		const lastStateMatchId = lastStateMatch?.id;
		const cached = this.lightweightCache.get(location);
		if (cached && cached[0] === lastStateMatchId) return cached[1];
		const [matchedRoutes, rawParams] = this.getMatchedRoutes(location.pathname);
		const lastRoute = require_utils.last(matchedRoutes);
		const accumulatedSearch = { ...location.search };
		for (const route of matchedRoutes) try {
			Object.assign(accumulatedSearch, validateSearch(route.options.validateSearch, accumulatedSearch));
		} catch {}
		const canReuseParams = lastStateMatch && lastStateMatch.routeId === lastRoute.id && lastStateMatch.pathname === location.pathname;
		let params;
		if (canReuseParams) params = lastStateMatch.params;
		else {
			const strictParams = Object.assign(Object.create(null), rawParams);
			for (const route of matchedRoutes) try {
				extractStrictParams(route, strictParams);
			} catch {}
			params = strictParams;
		}
		const result = [
			matchedRoutes,
			lastRoute.fullPath,
			accumulatedSearch,
			params
		];
		this.lightweightCache.set(location, [lastStateMatchId, result]);
		return result;
	}
};
/**
* In non-production environments,
* augment the RouterCore class with a `_refreshRoute` method
* dedicated to HMR.
*/
if (process.env.NODE_ENV !== "production") {
	RouterCore.prototype._replaceRouteChunk = require_load_client.replaceRouteChunk;
	RouterCore.prototype._refreshRoute = async function() {
		this._serverResult = void 0;
		this.updateLatestLocation();
		await require_load_client.refreshClientRoute(this);
	};
}
/** Error thrown when search parameter validation fails. */
var SearchParamError = class extends Error {};
/** Error thrown when path parameter parsing/validation fails. */
var PathParamError = class extends Error {};
const normalize = (str) => str.endsWith("/") && str.length > 1 ? str.slice(0, -1) : str;
function comparePaths(a, b) {
	return normalize(a) === normalize(b);
}
/**
* Lazily import a module function and forward arguments to it, retaining
* parameter and return types for the selected export key.
*/
function lazyFn(fn, key) {
	return async (...args) => {
		return (await fn())[key || "default"](...args);
	};
}
/** Create an initial RouterState from a parsed location. */
function getInitialRouterState(location) {
	return {
		isLoading: false,
		status: "idle",
		resolvedLocation: void 0,
		location,
		matches: []
	};
}
function validateSearch(validateSearch, input) {
	if (validateSearch == null) return {};
	if ("~standard" in validateSearch) {
		const result = validateSearch["~standard"].validate(input);
		if (result instanceof Promise) throw new SearchParamError("Async validation not supported");
		if (result.issues) throw new SearchParamError(JSON.stringify(result.issues, void 0, 2), { cause: result });
		return result.value;
	}
	if ("parse" in validateSearch) return validateSearch.parse(input);
	if (typeof validateSearch === "function") return validateSearch(input);
	return {};
}
function applySearchMiddleware(search, dest, destRoutes, includeValidateSearch) {
	const middlewares = [];
	for (const route of destRoutes) {
		const routeOptions = route.options;
		if ("search" in routeOptions) {
			if (routeOptions.search?.middlewares) middlewares.push(...routeOptions.search.middlewares);
		} else if (routeOptions.preSearchFilters || routeOptions.postSearchFilters) {
			const legacyMiddleware = ({ search, next }) => {
				const result = next(routeOptions.preSearchFilters ? routeOptions.preSearchFilters.reduce((prev, next) => next(prev), search) : search);
				return routeOptions.postSearchFilters ? routeOptions.postSearchFilters.reduce((prev, next) => next(prev), result) : result;
			};
			middlewares.push(legacyMiddleware);
		}
		const routeValidateSearch = routeOptions.validateSearch;
		if (routeValidateSearch) {
			const validate = ({ search, next, meta }) => {
				const result = next(search);
				if (includeValidateSearch) try {
					const validated = validateSearch(routeValidateSearch, result);
					if (meta && validated) {
						for (const key in validated) if (!(key in result)) (meta.defaulted ||= /* @__PURE__ */ new Map()).set(key, validated[key]);
					}
					return {
						...result,
						...validated
					};
				} catch {}
				return result;
			};
			middlewares.push(validate);
		}
	}
	const applyNext = (index, currentSearch, meta) => {
		if (index >= middlewares.length) {
			if (!dest.search) return {};
			if (dest.search === true) return currentSearch;
			const result = require_utils.functionalUpdate(dest.search, currentSearch);
			if (meta) meta.explicit = result;
			return result;
		}
		const next = (newSearch, collectMeta) => {
			if (collectMeta) {
				const nextMeta = meta || {};
				return {
					search: applyNext(index + 1, newSearch, nextMeta),
					meta: nextMeta
				};
			}
			return applyNext(index + 1, newSearch, meta);
		};
		return middlewares[index]({
			search: currentSearch,
			next,
			meta
		});
	};
	return applyNext(0, search);
}
function findGlobalNotFoundRouteId(notFoundMode, routes) {
	if (notFoundMode !== "root") {
		let fallback;
		for (let i = routes.length - 1; i >= 0; i--) {
			const route = routes[i];
			if (route.options.notFoundComponent) return route.id;
			fallback ||= route.children && route.id;
		}
		if (fallback) return fallback;
	}
	return require_root.rootRouteId;
}
function resolveNextParams(spec, base) {
	return spec === false || spec === null ? Object.create(null) : (spec ?? true) === true ? base : Object.assign(base, require_utils.functionalUpdate(spec, base));
}
function extractStrictParams(route, accumulatedParams) {
	const parseParams = route.options.params?.parse ?? route.options.parseParams;
	if (parseParams) Object.assign(accumulatedParams, parseParams(accumulatedParams));
}
//#endregion
exports.PathParamError = PathParamError;
exports.RouterCore = RouterCore;
exports.SearchParamError = SearchParamError;
exports.defaultSerializeError = defaultSerializeError;
exports.getInitialRouterState = getInitialRouterState;
exports.getLocationChangeInfo = getLocationChangeInfo;
exports.lazyFn = lazyFn;
exports.runRouteLifecycle = runRouteLifecycle;
exports.trailingSlashOptions = trailingSlashOptions;

//# sourceMappingURL=router.cjs.map