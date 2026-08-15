import React__default from "react";
const ShadowDomTargetContext = React__default.createContext(void 0);
const DevtoolsOnCloseContext = React__default.createContext(void 0);
const useDevtoolsOnClose = () => {
  const context = React__default.useContext(DevtoolsOnCloseContext);
  if (!context) {
    throw new Error(
      "useDevtoolsOnClose must be used within a TanStackRouterDevtools component"
    );
  }
  return context;
};
export {
  DevtoolsOnCloseContext,
  ShadowDomTargetContext,
  useDevtoolsOnClose
};
//# sourceMappingURL=context.js.map
