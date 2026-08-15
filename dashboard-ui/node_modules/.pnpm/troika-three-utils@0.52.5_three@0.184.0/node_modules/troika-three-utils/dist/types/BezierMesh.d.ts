/**
 * A ThreeJS `Mesh` that bends a tube shape along a 3D cubic bezier path. The bending is done
 * by deforming a straight cylindrical geometry in the vertex shader based on a set of four
 * control point uniforms. It patches the necessary GLSL into the mesh's assigned `material`
 * automatically.
 *
 * The cubiz bezier path is determined by its four `Vector3` properties:
 * - `pointA`
 * - `controlA`
 * - `controlB`
 * - `pointB`
 *
 * The tube's radius is controlled by its `radius` property, which defaults to `0.01`.
 *
 * You can also give the tube a dashed appearance with two properties:
 *
 * - `dashArray` - an array of two numbers, defining the length of "on" and "off" parts of
 *   the dash. Each is a 0-1 ratio of the entire path's length. (Actually this is the `t` length
 *   used as input to the cubic bezier function, not its visible length.)
 * - `dashOffset` - offset of where the dash starts. You can animate this to make the dashes move.
 *
 * Note that the dashes will appear like a hollow tube, not solid. This will be more apparent on
 * thicker tubes.
 *
 * TODO: proper geometry bounding sphere and raycasting
 * TODO: allow control of the geometry's segment counts
 */
export class BezierMesh {
    static getGeometry(): any;
    pointA: any;
    controlA: any;
    controlB: any;
    pointB: any;
    radius: number;
    dashArray: any;
    dashOffset: number;
    frustumCulled: boolean;
    set material(baseMaterial: THREE.Material);
    get material(): THREE.Material;
    _defaultMaterial: any;
    _derivedMaterial: any;
    _baseMaterial: any;
    get customDepthMaterial(): any;
    get customDistanceMaterial(): any;
    onBeforeRender(): void;
    raycast(): void;
}
//# sourceMappingURL=BezierMesh.d.ts.map