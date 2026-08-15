import { ErrorInfo } from 'react';
import { RegisteredRouter } from './router.js';
import { RouteIds } from './routeInfo.js';
import * as React from 'react';
export type NotFoundError = {
    /**
      @deprecated
      Use `routeId: rootRouteId` instead
    */
    global?: boolean;
    /**
      @private
      Do not use this. It's used internally to indicate a path matching error
    */
    _global?: boolean;
    data?: any;
    throw?: boolean;
    routeId?: RouteIds<RegisteredRouter['routeTree']>;
    headers?: HeadersInit;
};
export declare function notFound(options?: NotFoundError): NotFoundError;
export declare function isNotFound(obj: any): obj is NotFoundError;
export declare function CatchNotFound(props: {
    fallback?: (error: NotFoundError) => React.ReactElement;
    onCatch?: (error: Error, errorInfo: ErrorInfo) => void;
    children: React.ReactNode;
}): import("react/jsx-runtime").JSX.Element;
export declare function DefaultGlobalNotFound(): import("react/jsx-runtime").JSX.Element;
