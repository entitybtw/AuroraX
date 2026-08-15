import { unpluginRouterGeneratorFactory } from "./router-generator-plugin.js";
import { unpluginRouterCodeSplitterFactory } from "./router-code-splitter-plugin.js";
const unpluginRouterComposedFactory = (options = {}, meta) => {
  const routerGenerator = unpluginRouterGeneratorFactory(options);
  const routerGeneratorOptions = Array.isArray(routerGenerator) ? routerGenerator : [routerGenerator];
  const routerCodeSplitter = unpluginRouterCodeSplitterFactory(options, meta);
  const routerCodeSplitterOptions = Array.isArray(routerCodeSplitter) ? routerCodeSplitter : [routerCodeSplitter];
  return [...routerGeneratorOptions, ...routerCodeSplitterOptions];
};
export {
  unpluginRouterComposedFactory
};
//# sourceMappingURL=router-composed-plugin.js.map
