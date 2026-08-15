export default workerModule;
export type ParsedFont = {
    ascender: number;
    descender: number;
    xHeight: number;
    supportsCodePoint: (number: any) => boolean;
    forEachGlyph: (text: string, fontSize: number, letterSpacing: number, callback: any) => number;
    lineGap: number;
    capHeight: number;
    unitsPerEm: number;
};
export type FontParser = (buffer: ArrayBuffer) => ParsedFont;
declare const workerModule: any;
//# sourceMappingURL=FontParser.d.ts.map