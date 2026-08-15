import { createEsbuildPlugin } from "unplugin";
import { configSchema } from "./core/config.js";
import { unpluginRouterCodeSplitterFactory } from "./core/router-code-splitter-plugin.js";
import { unpluginRouterGeneratorFactory } from "./core/router-generator-plugin.js";
import { unpluginRouterComposedFactory } from "./core/router-composed-plugin.js";
const TanStackRouterGeneratorEsbuild = createEsbuildPlugin(
  unpluginRouterGeneratorFactory
);
const TanStackRouterCodeSplitterEsbuild = createEsbuildPlugin(
  unpluginRouterCodeSplitterFactory
);
const TanStackRouterEsbuild = createEsbuildPlugin(unpluginRouterComposedFactory);
export {
  TanStackRouterCodeSplitterEsbuild,
  TanStackRouterEsbuild,
  TanStackRouterGeneratorEsbuild,
  configSchema,
  TanStackRouterEsbuild as default
};
//# sourceMappingURL=esbuild.js.map
