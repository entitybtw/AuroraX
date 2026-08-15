/**
 * A utility for creating a custom shader material derived from another material's
 * shaders. This allows you to inject custom shader logic and transforms into the
 * builtin ThreeJS materials without having to recreate them from scratch.
 *
 * @param {THREE.Material} baseMaterial - the original material to derive from
 *
 * @param {Object} options - How the base material should be modified.
 * @param {Object=} options.defines - Custom `defines` for the material
 * @param {Object=} options.extensions - Custom `extensions` for the material, e.g. `{derivatives: true}`
 * @param {Object=} options.uniforms - Custom `uniforms` for use in the modified shader. These can
 *        be accessed and manipulated via the resulting material's `uniforms` property, just like
 *        in a ShaderMaterial. You do not need to repeat the base material's own uniforms here.
 * @param {String=} options.timeUniform - If specified, a uniform of this name will be injected into
 *        both shaders, and it will automatically be updated on each render frame with a number of
 *        elapsed milliseconds. The "zero" epoch time is not significant so don't rely on this as a
 *        true calendar time.
 * @param {String=} options.vertexDefs - Custom GLSL code to inject into the vertex shader's top-level
 *        definitions, above the `void main()` function.
 * @param {String=} options.vertexMainIntro - Custom GLSL code to inject at the top of the vertex
 *        shader's `void main` function.
 * @param {String=} options.vertexMainOutro - Custom GLSL code to inject at the end of the vertex
 *        shader's `void main` function.
 * @param {String=} options.vertexTransform - Custom GLSL code to manipulate the `position`, `normal`,
 *        and/or `uv` vertex attributes. This code will be wrapped within a standalone function with
 *        those attributes exposed by their normal names as read/write values.
 * @param {String=} options.fragmentDefs - Custom GLSL code to inject into the fragment shader's top-level
 *        definitions, above the `void main()` function.
 * @param {String=} options.fragmentMainIntro - Custom GLSL code to inject at the top of the fragment
 *        shader's `void main` function.
 * @param {String=} options.fragmentMainOutro - Custom GLSL code to inject at the end of the fragment
 *        shader's `void main` function. You can manipulate `gl_FragColor` here but keep in mind it goes
 *        after any of ThreeJS's color postprocessing shader chunks (tonemapping, fog, etc.), so if you
 *        want those to apply to your changes use `fragmentColorTransform` instead.
 * @param {String=} options.fragmentColorTransform - Custom GLSL code to manipulate the `gl_FragColor`
 *        output value. Will be injected near the end of the `void main` function, but before any
 *        of ThreeJS's color postprocessing shader chunks (tonemapping, fog, etc.), and before the
 *        `fragmentMainOutro`.
 * @param {function({fragmentShader: string, vertexShader:string}):
 *        {fragmentShader: string, vertexShader:string}} options.customRewriter - A function
 *        for performing custom rewrites of the full shader code. Useful if you need to do something
 *        special that's not covered by the other builtin options. This function will be executed before
 *        any other transforms are applied.
 * @param {boolean=} options.chained - Set to `true` to prototype-chain the derived material to the base
 *        material, rather than the default behavior of copying it. This allows the derived material to
 *        automatically pick up changes made to the base material and its properties. This can be useful
 *        where the derived material is hidden from the user as an implementation detail, allowing them
 *        to work with the original material like normal. But it can result in unexpected behavior if not
 *        handled carefully.
 *
 * @return {THREE.Material}
 *
 * The returned material will also have two new methods, `getDepthMaterial()` and `getDistanceMaterial()`,
 * which can be called to get a variant of the derived material for use in shadow casting. If the
 * target mesh is expected to cast shadows, then you can assign these to the mesh's `customDepthMaterial`
 * (for directional and spot lights) and/or `customDistanceMaterial` (for point lights) properties to
 * allow the cast shadow to honor your derived shader's vertex transforms and discarded fragments. These
 * will also set a custom `#define IS_DEPTH_MATERIAL` or `#define IS_DISTANCE_MATERIAL` that you can look
 * for in your derived shaders with `#ifdef` to customize their behavior for the depth or distance
 * scenarios, e.g. skipping antialiasing or expensive shader logic.
 */
export function createDerivedMaterial(baseMaterial: THREE.Material, options: {
    defines?: any | undefined;
    extensions?: any | undefined;
    uniforms?: any | undefined;
    timeUniform?: string | undefined;
    vertexDefs?: string | undefined;
    vertexMainIntro?: string | undefined;
    vertexMainOutro?: string | undefined;
    vertexTransform?: string | undefined;
    fragmentDefs?: string | undefined;
    fragmentMainIntro?: string | undefined;
    fragmentMainOutro?: string | undefined;
    fragmentColorTransform?: string | undefined;
    customRewriter: (arg0: {
        fragmentShader: string;
        vertexShader: string;
    }) => {
        fragmentShader: string;
        vertexShader: string;
    };
    chained?: boolean | undefined;
}): THREE.Material;
//# sourceMappingURL=DerivedMaterial.d.ts.map