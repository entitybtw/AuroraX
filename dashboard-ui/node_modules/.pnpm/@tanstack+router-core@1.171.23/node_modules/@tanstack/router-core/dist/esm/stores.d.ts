import { AnyRoute } from './route.js';
import { RouterState } from './router.js';
import { FullSearchSchema } from './routeInfo.js';
import { ParsedLocation } from './location.js';
import { AnyRouteMatch } from './Matches.js';
export interface RouterReadableStore<TValue> {
    get: () => TValue;
}
export interface RouterWritableStore<TValue> extends RouterReadableStore<TValue> {
    set: ((updater: (prev: TValue) => TValue) => void) & ((value: TValue) => void);
}
export type RouterBatchFn = (fn: () => void) => void;
export type MutableStoreFactory = <TValue>(initialValue: TValue) => RouterWritableStore<TValue>;
export type ReadonlyStoreFactory = <TValue>(read: () => TValue) => RouterReadableStore<TValue>;
export type GetStoreConfig = (opts: {
    isServer?: boolean;
}) => StoreConfig;
export type StoreConfig = {
    createMutableStore: MutableStoreFactory;
    createReadonlyStore: ReadonlyStoreFactory;
    batch: RouterBatchFn;
};
type MatchStore = RouterWritableStore<AnyRouteMatch | undefined>;
type ReadableStore<TValue> = RouterReadableStore<TValue>;
/** SSR non-reactive createMutableStore */
export declare function createNonReactiveMutableStore<TValue>(initialValue: TValue): RouterWritableStore<TValue>;
/** SSR non-reactive createReadonlyStore */
export declare function createNonReactiveReadonlyStore<TValue>(read: () => TValue): RouterReadableStore<TValue>;
export interface RouterStores<in out TRouteTree extends AnyRoute> {
    status: RouterWritableStore<RouterState<TRouteTree>['status']>;
    location: RouterWritableStore<ParsedLocation<FullSearchSchema<TRouteTree>>>;
    resolvedLocation: RouterWritableStore<ParsedLocation<FullSearchSchema<TRouteTree>> | undefined>;
    ids: RouterWritableStore<Array<string>>;
    matches: ReadableStore<Array<AnyRouteMatch>>;
    __store: RouterReadableStore<RouterState<TRouteTree>>;
    byRoute: Map<string, MatchStore>;
    /**
     * Get the stable atom for a route's presented match. The atom remains in the
     * pool when the route leaves and contains `undefined` until it re-enters.
     */
    getMatchStore: (routeId: string) => RouterReadableStore<AnyRouteMatch | undefined>;
    setMatches: (nextMatches: Array<AnyRouteMatch>) => void;
}
export declare function createRouterStores<TRouteTree extends AnyRoute>(initialLocation: RouterState<TRouteTree>['location'], config: StoreConfig): RouterStores<TRouteTree>;
export {};
