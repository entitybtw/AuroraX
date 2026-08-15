import { AnyRouteMatch } from './Matches.js';
import { AnyRedirect } from './redirect.js';
import { AnyRouter } from './router.js';
export type ServerLoadResult = {
    type: 'render';
    status: 200 | 404 | 500;
    matches: Array<AnyRouteMatch>;
} | {
    type: 'redirect';
    redirect: AnyRedirect;
};
type ServerLoadOptions = NonNullable<Parameters<AnyRouter['load']>[0]> & {
    _signal?: AbortSignal;
};
export declare function loadServerRoute(router: AnyRouter, opts?: ServerLoadOptions): Promise<void>;
export {};
