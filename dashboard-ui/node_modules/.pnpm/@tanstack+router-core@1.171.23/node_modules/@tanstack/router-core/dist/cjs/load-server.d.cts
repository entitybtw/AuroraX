import { AnyRouteMatch } from './Matches.cjs';
import { AnyRedirect } from './redirect.cjs';
import { AnyRouter } from './router.cjs';
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
