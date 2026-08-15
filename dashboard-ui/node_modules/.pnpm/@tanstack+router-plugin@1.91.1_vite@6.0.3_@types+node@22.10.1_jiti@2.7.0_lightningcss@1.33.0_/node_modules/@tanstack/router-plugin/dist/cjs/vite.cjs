"use strict";
Object.defineProperties(exports, { __esModule: { value: true }, [Symbol.toStringTag]: { value: "Module" } });
const unplugin = require("unplugin");
const config = require("./core/config.cjs");
const routerCodeSplitterPlugin = require("./core/router-code-splitter-plugin.cjs");
const routerGeneratorPlugin = require("./core/router-generator-plugin.cjs");
const routerComposedPlugin = require("./core/router-composed-plugin.cjs");
const TanStackRouterGeneratorVite = unplugin.createVitePlugin(
  routerGeneratorPlugin.unpluginRouterGeneratorFactory
);
const TanStackRouterCodeSplitterVite = unplugin.createVitePlugin(
  routerCodeSplitterPlugin.unpluginRouterCodeSplitterFactory
);
const TanStackRouterVite = unplugin.createVitePlugin(routerComposedPlugin.unpluginRouterComposedFactory);
exports.configSchema = config.configSchema;
exports.TanStackRouterCodeSplitterVite = TanStackRouterCodeSplitterVite;
exports.TanStackRouterGeneratorVite = TanStackRouterGeneratorVite;
exports.TanStackRouterVite = TanStackRouterVite;
exports.default = TanStackRouterVite;
//# sourceMappingURL=vite.cjs.map
