/**
 * @typedef {object} TextCaret
 * @property {number} x - x position of the caret
 * @property {number} y - y position of the caret's bottom
 * @property {number} height - height of the caret
 * @property {number} charIndex - the index in the original input string of this caret's target
 *   character; the caret will be for the position _before_ that character.
 */
/**
 * Given a local x/y coordinate in the text block plane, find the nearest caret position.
 * @param {TroikaTextRenderInfo} textRenderInfo - a result object from TextBuilder#getTextRenderInfo
 * @param {number} x
 * @param {number} y
 * @return {TextCaret | null}
 */
export function getCaretAtPoint(textRenderInfo: TroikaTextRenderInfo, x: number, y: number): TextCaret | null;
/**
 * Given start and end character indexes, return a list of rectangles covering all the
 * characters within that selection.
 * @param {TroikaTextRenderInfo} textRenderInfo
 * @param {number} start - index of the first char in the selection
 * @param {number} end - index of the first char after the selection
 * @return {Array<{left, top, right, bottom}> | null}
 */
export function getSelectionRects(textRenderInfo: TroikaTextRenderInfo, start: number, end: number): Array<{
    left: any;
    top: any;
    right: any;
    bottom: any;
}> | null;
export type TextCaret = {
    /**
     * - x position of the caret
     */
    x: number;
    /**
     * - y position of the caret's bottom
     */
    y: number;
    /**
     * - height of the caret
     */
    height: number;
    /**
     * - the index in the original input string of this caret's target
     * character; the caret will be for the position _before_ that character.
     */
    charIndex: number;
};
//# sourceMappingURL=selectionUtils.d.ts.map