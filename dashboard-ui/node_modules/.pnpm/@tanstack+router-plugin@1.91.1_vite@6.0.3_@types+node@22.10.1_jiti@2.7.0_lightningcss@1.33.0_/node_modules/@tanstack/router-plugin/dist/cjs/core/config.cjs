"use strict";
Object.defineProperty(exports, Symbol.toStringTag, { value: "Module" });
const zod = require("zod");
const routerGenerator = require("@tanstack/router-generator");
const configSchema = routerGenerator.configSchema.extend({
  enableRouteGeneration: zod.z.boolean().optional()
});
const getConfig = (inlineConfig, root) => {
  const config = routerGenerator.getConfig(inlineConfig, root);
  return configSchema.parse({ ...config, ...inlineConfig });
};
exports.configSchema = configSchema;
exports.getConfig = getConfig;
//# sourceMappingURL=config.cjs.map
