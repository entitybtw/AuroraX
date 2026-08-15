import { ParsedLocation } from './location.js';
export type ScrollRestorationOptions = {
    getKey?: (location: ParsedLocation) => string;
};
export declare function useScrollRestoration(options?: ScrollRestorationOptions): void;
export declare function ScrollRestoration(props: ScrollRestorationOptions): null;
export declare function useElementScrollRestoration(options: ({
    id: string;
    getElement?: () => Element | undefined | null;
} | {
    id?: string;
    getElement: () => Element | undefined | null;
}) & {
    getKey?: (location: ParsedLocation) => string;
}): {
    scrollX: number;
    scrollY: number;
} | undefined;
