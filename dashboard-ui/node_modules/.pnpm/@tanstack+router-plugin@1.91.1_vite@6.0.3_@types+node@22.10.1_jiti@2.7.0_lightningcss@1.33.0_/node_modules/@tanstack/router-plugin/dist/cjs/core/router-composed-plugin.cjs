"use strict";
Object.defineProperty(exports, Symbol.toStringTag, { value: "Module" });
const routerGeneratorPlugin = require("./router-generator-plugin.cjs");
const routerCodeSplitterPlugin = require("./router-code-splitter-plugin.cjs");
const unpluginRouterComposedFactory = (options = {}, meta) => {
  const routerGenerator = routerGeneratorPlugin.unpluginRouterGeneratorFactory(options);
  const routerGeneratorOptions = Array.isArray(routerGenerator) ? routerGenerator : [routerGenerator];
  const routerCodeSplitter = routerCodeSplitterPlugin.unpluginRouterCodeSplitterFactory(options, meta);
  const routerCodeSplitterOptions = Array.isArray(routerCodeSplitter) ? routerCodeSplitter : [routerCodeSplitter];
  return [...routerGeneratorOptions, ...routerCodeSplitterOptions];
};
exports.unpluginRouterComposedFactory = unpluginRouterComposedFactory;
//# sourceMappingURL=router-composed-plugin.cjs.map
