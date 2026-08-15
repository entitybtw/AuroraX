import { z } from "zod";
import { configSchema as configSchema$1, getConfig as getConfig$1 } from "@tanstack/router-generator";
const configSchema = configSchema$1.extend({
  enableRouteGeneration: z.boolean().optional()
});
const getConfig = (inlineConfig, root) => {
  const config = getConfig$1(inlineConfig, root);
  return configSchema.parse({ ...config, ...inlineConfig });
};
export {
  configSchema,
  getConfig
};
//# sourceMappingURL=config.js.map
