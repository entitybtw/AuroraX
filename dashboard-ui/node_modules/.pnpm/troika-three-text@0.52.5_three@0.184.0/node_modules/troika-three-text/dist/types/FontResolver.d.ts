/**
 * @typedef {string | {src:string, label?:string, unicodeRange?:string, lang?:string}} UserFont
 */
/**
 * @typedef {ClientOptions} FontResolverOptions
 * @property {Array<UserFont>|UserFont} [fonts]
 * @property {'normal'|'italic'} [style]
 * @property {'normal'|'bold'|number} [style]
 * @property {string} [unicodeFontsURL]
 */
/**
 * @typedef {Object} FontResolverResult
 * @property {Uint8Array} chars
 * @property {Array<ParsedFont & {src:string}>} fonts
 */
/**
 * @typedef {function} FontResolver
 * @param {string} text
 * @param {(FontResolverResult) => void} callback
 * @param {FontResolverOptions} [options]
 */
/**
 * Factory for the FontResolver function.
 * @param {FontParser} fontParser
 * @param {{getFontsForString: function, CodePointSet: function}} unicodeFontResolverClient
 * @return {FontResolver}
 */
export function createFontResolver(fontParser: FontParser, unicodeFontResolverClient: {
    getFontsForString: Function;
    CodePointSet: Function;
}): FontResolver;
export const fontResolverWorkerModule: any;
export type UserFont = string | {
    src: string;
    label?: string;
    unicodeRange?: string;
    lang?: string;
};
export type FontResolverOptions = ClientOptions;
export type FontResolverResult = {
    chars: Uint8Array;
    fonts: Array<ParsedFont & {
        src: string;
    }>;
};
export type FontResolver = Function;
//# sourceMappingURL=FontResolver.d.ts.map