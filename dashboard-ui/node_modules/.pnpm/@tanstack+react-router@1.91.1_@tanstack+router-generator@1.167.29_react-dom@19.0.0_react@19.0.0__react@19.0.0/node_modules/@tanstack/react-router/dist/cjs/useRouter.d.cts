import { AnyRouter, RegisteredRouter } from './router.cjs';
export declare function useRouter<TRouter extends AnyRouter = RegisteredRouter>(opts?: {
    warn?: boolean;
}): TRouter;
