import * as React from 'react';
type ExpanderProps = {
    expanded: boolean;
    style?: React.CSSProperties;
};
export declare const Expander: ({ expanded, style }: ExpanderProps) => import("react/jsx-runtime").JSX.Element;
type Entry = {
    label: string;
};
type RendererProps = {
    handleEntry: HandleEntryFn;
    label?: React.ReactNode;
    value: unknown;
    subEntries: Array<Entry>;
    subEntryPages: Array<Array<Entry>>;
    type: string;
    expanded: boolean;
    toggleExpanded: () => void;
    pageSize: number;
    renderer?: Renderer;
    filterSubEntries?: (subEntries: Array<Property>) => Array<Property>;
};
/**
 * Chunk elements in the array by size
 *
 * when the array cannot be chunked evenly by size, the last chunk will be
 * filled with the remaining elements
 *
 * @example
 * chunkArray(['a','b', 'c', 'd', 'e'], 2) // returns [['a','b'], ['c', 'd'], ['e']]
 */
export declare function chunkArray<T>(array: Array<T>, size: number): Array<Array<T>>;
type Renderer = (props: RendererProps) => React.JSX.Element;
export declare const DefaultRenderer: Renderer;
type HandleEntryFn = (entry: Entry) => React.ReactNode;
type ExplorerProps = Partial<RendererProps> & {
    renderer?: Renderer;
    defaultExpanded?: true | Record<string, boolean>;
};
type Property = {
    defaultExpanded?: boolean | Record<string, boolean>;
    label: string;
    value: unknown;
};
export default function Explorer({ value, defaultExpanded, renderer, pageSize, filterSubEntries, ...rest }: ExplorerProps): React.JSX.Element;
export {};
