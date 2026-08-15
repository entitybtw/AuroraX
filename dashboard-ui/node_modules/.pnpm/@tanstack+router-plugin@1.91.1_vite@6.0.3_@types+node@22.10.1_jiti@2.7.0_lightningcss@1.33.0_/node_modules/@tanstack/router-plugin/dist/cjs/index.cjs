"use strict";
Object.defineProperty(exports, Symbol.toStringTag, { value: "Module" });
const config = require("./core/config.cjs");
const routerCodeSplitterPlugin = require("./core/router-code-splitter-plugin.cjs");
const routerGeneratorPlugin = require("./core/router-generator-plugin.cjs");
exports.configSchema = config.configSchema;
exports.unpluginRouterCodeSplitterFactory = routerCodeSplitterPlugin.unpluginRouterCodeSplitterFactory;
exports.unpluginRouterGeneratorFactory = routerGeneratorPlugin.unpluginRouterGeneratorFactory;
//# sourceMappingURL=index.cjs.map
