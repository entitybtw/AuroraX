/**
 * Helper for smoothing out the `m.getInverse(x)` --> `m.copy(x).invert()` conversion
 * that happened in ThreeJS r123.
 * @param {Matrix4} srcMatrix
 * @param {Matrix4} [tgtMatrix]
 */
export function invertMatrix4(srcMatrix: Matrix4, tgtMatrix?: Matrix4): Matrix4;
//# sourceMappingURL=invertMatrix4.d.ts.map