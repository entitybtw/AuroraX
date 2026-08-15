import { AnyRouter, RegisteredRouter } from './router.js';
export declare function useRouter<TRouter extends AnyRouter = RegisteredRouter>(opts?: {
    warn?: boolean;
}): TRouter;
