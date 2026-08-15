import { RouteNode } from '../types.js';
import { Config } from '../config.js';
export interface TransformOptions {
    source: string;
    filename?: string;
    ctx: TransformContext;
    node: RouteNode;
}
export type TransformResult = {
    result: 'no-route-export';
} | {
    result: 'not-modified';
} | {
    result: 'modified';
    output: string;
} | {
    result: 'error';
    error?: any;
};
export interface TransformContext {
    target: Config['target'];
    routeId: string;
    lazy: boolean;
}
