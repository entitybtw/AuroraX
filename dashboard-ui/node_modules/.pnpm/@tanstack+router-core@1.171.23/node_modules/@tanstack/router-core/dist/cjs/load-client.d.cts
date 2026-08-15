import { GLOBAL_SEROVAL, GLOBAL_TSR } from './ssr/constants.cjs';
import { TsrSsrGlobal } from './ssr/types.cjs';
import { ParsedLocation } from './location.cjs';
import { AnyRouteMatch } from './Matches.cjs';
import { NotFoundError } from './not-found.cjs';
import { AnyRoute } from './route.cjs';
import { AnyRedirect } from './redirect.cjs';
import { AnyRouter } from './router.cjs';
export declare function replaceRouteChunk(route: AnyRoute, lazyFn: AnyRoute['lazyFn']): void;
export declare function loadRouteChunk(route: AnyRoute, componentType?: 'errorComponent' | 'notFoundComponent' | false, onPendingReady?: () => void): Promise<void> | undefined;
/** Return the structural lane through the first terminal render boundary. */
export declare function _getRenderedMatches(matches: Array<AnyRouteMatch>): Array<AnyRouteMatch>;
/** Return the lane whose document assets belong to the current presentation. */
export declare function _getAssetMatches(matches: Array<AnyRouteMatch>): Array<AnyRouteMatch>;
declare const lanePhase: unique symbol;
type LanePhase = 'matched' | 'contextualized' | 'reduced' | 'projected';
/**
 * Lane matches carry their lane's phase so functions can demand evidence of
 * pipeline position (e.g. `commitMatches` only accepts a projected lane's
 * matches). The brand is phantom — it never exists at runtime.
 */
type LaneMatches<TPhase extends LanePhase> = Array<WorkMatch> & {
    readonly [lanePhase]?: TPhase;
};
type Lane<TPhase extends LanePhase> = [
    location: ParsedLocation,
    matches: LaneMatches<TPhase>,
    background?: Array<BackgroundLoaderTask>,
    backgroundSettlement?: Promise<IndexedOutcome | undefined>
] & {
    readonly [lanePhase]?: TPhase;
};
type ReducedLane = Lane<'reduced'>;
type ProjectedLane = Lane<'projected'>;
declare const SUCCESS = 0;
declare const ERROR = 1;
declare const NOT_FOUND = 2;
declare const REDIRECTED = 3;
declare const CANCELED = 4;
type LoaderOutcome = [kind: typeof SUCCESS, data: unknown] | [kind: typeof ERROR, error: unknown] | [kind: typeof NOT_FOUND, error: NotFoundError] | [kind: typeof REDIRECTED, redirect: AnyRedirect] | [kind: typeof CANCELED];
type IndexedOutcome = [index: number, outcome: LoaderOutcome, boundary?: number];
export type LoaderFlight = [
    outcome: Promise<LoaderOutcome>,
    controller: AbortController,
    leases: number
];
type WorkMatch = AnyRouteMatch & {
    _flight?: LoaderFlight;
};
declare const matchPhase: unique symbol;
/**
 * A match whose loader outcome has been applied by `settleInto`, which is the
 * sole granter of this brand (phantom, zero-runtime). Consumers that require
 * it — e.g. `cacheLoaderMatch` — can only be reached after settlement, so the
 * compiler enforces the loader→settle→cache ordering. Sources that arrive
 * already settled (dehydrated server data) must cast at a named boundary.
 */
type SettledMatch = WorkMatch & {
    readonly [matchPhase]: 'settled';
};
export type LoadTransaction = [
    controller: AbortController,
    redirects: number,
    location: ParsedLocation,
    matches: Array<AnyRouteMatch>,
    startedAt: number,
    done: Promise<void>,
    /**
     * Dev-only HMR refresh mode. Presence is the mode flag; a refresh always
     * carries the presentation it started from and its optional hydration
     * handoff. While a publication awaits acknowledgement, its rollback lives
     * with the transaction that owns the publication.
     */
    refresh?: [
        presentation: Array<AnyRouteMatch>,
        handoff: NonNullable<AnyRouter['_handoff']> | undefined,
        rollback?: () => boolean
    ]
];
export type PendingSession = [
    owner: LoadTransaction,
    boundary: number,
    /** Pending reveal time until acknowledged, then minimum-visible-until time. */
    deadline: number,
    timer?: ReturnType<typeof setTimeout>,
    ack?: Promise<boolean>,
    component?: unknown
];
type CoordinatorRouter = AnyRouter & {
    /** Active speculative lanes retained for cancellation, invalidation, and cache clearing. */
    _preloads?: Map<AbortController, Array<AnyRouteMatch>>;
    _refreshNextLoad?: boolean;
    _cancelTransition?: () => void;
};
type BackgroundLoaderTask = [
    index: number,
    outcome: Promise<LoaderOutcome>,
    chunkFailure: Promise<IndexedOutcome | undefined>,
    candidate: WorkMatch
];
export declare function waitFor<T>(value: T | PromiseLike<T>, signal: AbortSignal): Promise<T>;
export declare function getRoute(router: AnyRouter, match: WorkMatch): AnyRoute;
export declare function navigateFrom(router: AnyRouter, location: ParsedLocation): (opts: any) => Promise<void>;
export declare function cacheLoaderMatch(router: CoordinatorRouter, match: SettledMatch, planned: AnyRouteMatch | undefined): void;
export declare function projectLane(router: AnyRouter, lane: ReducedLane, signal: AbortSignal, start?: number, end?: number): Promise<ProjectedLane>;
export declare function loadClientRoute(router: CoordinatorRouter, opts?: {
    sync?: boolean;
}): Promise<void>;
export declare function refreshClientRoute(router: CoordinatorRouter): Promise<void>;
export declare function preloadClientRoute(router: CoordinatorRouter, opts: any, redirects?: number): Promise<Array<AnyRouteMatch> | undefined>;
declare global {
    interface Window {
        [GLOBAL_TSR]?: TsrSsrGlobal;
        [GLOBAL_SEROVAL]?: any;
    }
}
export declare function hydrate(router: AnyRouter): Promise<void>;
export {};
