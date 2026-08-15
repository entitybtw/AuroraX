/**
 * @typedef {number|'left'|'center'|'right'} AnchorXValue
 */
/**
 * @typedef {number|'top'|'top-baseline'|'top-cap'|'top-ex'|'middle'|'bottom-baseline'|'bottom'} AnchorYValue
 */
/**
 * @typedef {object} TypesetParams
 * @property {string} text
 * @property {UserFont|UserFont[]} [font]
 * @property {string} [lang]
 * @property {number} [sdfGlyphSize=64]
 * @property {number} [fontSize=1]
 * @property {number|'normal'|'bold'} [fontWeight='normal']
 * @property {'normal'|'italic'} [fontStyle='normal']
 * @property {number} [letterSpacing=0]
 * @property {'normal'|number} [lineHeight='normal']
 * @property {number} [maxWidth]
 * @property {'ltr'|'rtl'} [direction='ltr']
 * @property {string} [textAlign='left']
 * @property {number} [textIndent=0]
 * @property {'normal'|'nowrap'} [whiteSpace='normal']
 * @property {'normal'|'break-word'} [overflowWrap='normal']
 * @property {AnchorXValue} [anchorX=0]
 * @property {AnchorYValue} [anchorY=0]
 * @property {boolean} [metricsOnly=false]
 * @property {string} [unicodeFontsURL]
 * @property {FontResolverResult} [preResolvedFonts]
 * @property {boolean} [includeCaretPositions=false]
 * @property {number} [chunkedBoundsSize=8192]
 * @property {{[rangeStartIndex]: number}} [colorRanges]
 */
/**
 * @typedef {object} TypesetResult
 * @property {Uint16Array} glyphIds id for each glyph, specific to that glyph's font
 * @property {Uint8Array} glyphFontIndices index into fontData for each glyph
 * @property {Float32Array} glyphPositions x,y of each glyph's origin in layout
 * @property {{[font]: {[glyphId]: {path: string, pathBounds: number[]}}}} glyphData data about each glyph appearing in the text
 * @property {TypesetFontData[]} fontData data about each font used in the text
 * @property {Float32Array} [caretPositions] startX,endX,bottomY caret positions for each char
 * @property {Uint8Array} [glyphColors] color for each glyph, if color ranges supplied
 *         chunkedBounds, //total rects per (n=chunkedBoundsSize) consecutive glyphs
 *         fontSize, //calculated em height
 *         topBaseline: anchorYOffset + lines[0].baseline, //y coordinate of the top line's baseline
 *         blockBounds: [ //bounds for the whole block of text, including vertical padding for lineHeight
 *           anchorXOffset,
 *           anchorYOffset - totalHeight,
 *           anchorXOffset + maxLineWidth,
 *           anchorYOffset
 *         ],
 *         visibleBounds, //total bounds of visible text paths, may be larger or smaller than blockBounds
 *         timings
 */
/**
 * @typedef {object} TypesetFontData
 * @property src
 * @property unitsPerEm
 * @property ascender
 * @property descender
 * @property lineHeight
 * @property capHeight
 * @property xHeight
 */
/**
 * @typedef {function} TypesetterTypesetFunction - compute fonts and layout for some text.
 * @param {TypesetParams} params
 * @param {(TypesetResult) => void} callback - function called when typesetting is complete.
 *    If the params included `preResolvedFonts`, this will be called synchronously.
 */
/**
 * @typedef {function} TypesetterMeasureFunction - compute width/height for some text.
 * @param {TypesetParams} params
 * @param {(width:number, height:number) => void} callback - function called when measurement is complete.
 *    If the params included `preResolvedFonts`, this will be called synchronously.
 */
/**
 * Factory function that creates a self-contained environment for processing text typesetting requests.
 *
 * It is important that this function has no closure dependencies, so that it can be easily injected
 * into the source for a Worker without requiring a build step or complex dependency loading. All its
 * dependencies must be passed in at initialization.
 *
 * @param {FontResolver} resolveFonts - function to resolve a string to parsed fonts
 * @param {object} bidi - the bidi.js implementation object
 * @return {{typeset: TypesetterTypesetFunction, measure: TypesetterMeasureFunction}}
 */
export function createTypesetter(resolveFonts: FontResolver, bidi: object): {
    typeset: TypesetterTypesetFunction;
    measure: TypesetterMeasureFunction;
};
export type AnchorXValue = number | "left" | "center" | "right";
export type AnchorYValue = number | "top" | "top-baseline" | "top-cap" | "top-ex" | "middle" | "bottom-baseline" | "bottom";
export type TypesetParams = {
    text: string;
    font?: UserFont | UserFont[];
    lang?: string;
    sdfGlyphSize?: number;
    fontSize?: number;
    fontWeight?: number | "normal" | "bold";
    fontStyle?: "normal" | "italic";
    letterSpacing?: number;
    lineHeight?: "normal" | number;
    maxWidth?: number;
    direction?: "ltr" | "rtl";
    textAlign?: string;
    textIndent?: number;
    whiteSpace?: "normal" | "nowrap";
    overflowWrap?: "normal" | "break-word";
    anchorX?: AnchorXValue;
    anchorY?: AnchorYValue;
    metricsOnly?: boolean;
    unicodeFontsURL?: string;
    preResolvedFonts?: FontResolverResult;
    includeCaretPositions?: boolean;
    chunkedBoundsSize?: number;
    colorRanges?: {
        [rangeStartIndex]: number;
    };
};
export type TypesetResult = {
    /**
     * id for each glyph, specific to that glyph's font
     */
    glyphIds: Uint16Array;
    /**
     * index into fontData for each glyph
     */
    glyphFontIndices: Uint8Array;
    /**
     * x,y of each glyph's origin in layout
     */
    glyphPositions: Float32Array;
    /**
     * data about each glyph appearing in the text
     */
    glyphData: {
        [font]: {
            [glyphId]: {
                path: string;
                pathBounds: number[];
            };
        };
    };
    /**
     * data about each font used in the text
     */
    fontData: TypesetFontData[];
    /**
     * startX,endX,bottomY caret positions for each char
     */
    caretPositions?: Float32Array;
    /**
     * color for each glyph, if color ranges supplied
     * chunkedBounds, //total rects per (n=chunkedBoundsSize) consecutive glyphs
     * fontSize, //calculated em height
     * topBaseline: anchorYOffset + lines[0].baseline, //y coordinate of the top line's baseline
     * blockBounds: [ //bounds for the whole block of text, including vertical padding for lineHeight
     * anchorXOffset,
     * anchorYOffset - totalHeight,
     * anchorXOffset + maxLineWidth,
     * anchorYOffset
     * ],
     * visibleBounds, //total bounds of visible text paths, may be larger or smaller than blockBounds
     * timings
     */
    glyphColors?: Uint8Array;
};
export type TypesetFontData = {
    src: any;
    unitsPerEm: any;
    ascender: any;
    descender: any;
    lineHeight: any;
    capHeight: any;
    xHeight: any;
};
/**
 * - compute fonts and layout for some text.
 */
export type TypesetterTypesetFunction = Function;
/**
 * - compute width/height for some text.
 */
export type TypesetterMeasureFunction = Function;
//# sourceMappingURL=Typesetter.d.ts.map