import { createVitePlugin } from "unplugin";
import { configSchema } from "./core/config.js";
import { unpluginRouterCodeSplitterFactory } from "./core/router-code-splitter-plugin.js";
import { unpluginRouterGeneratorFactory } from "./core/router-generator-plugin.js";
import { unpluginRouterComposedFactory } from "./core/router-composed-plugin.js";
const TanStackRouterGeneratorVite = createVitePlugin(
  unpluginRouterGeneratorFactory
);
const TanStackRouterCodeSplitterVite = createVitePlugin(
  unpluginRouterCodeSplitterFactory
);
const TanStackRouterVite = createVitePlugin(unpluginRouterComposedFactory);
export {
  TanStackRouterCodeSplitterVite,
  TanStackRouterGeneratorVite,
  TanStackRouterVite,
  configSchema,
  TanStackRouterVite as default
};
//# sourceMappingURL=vite.js.map
