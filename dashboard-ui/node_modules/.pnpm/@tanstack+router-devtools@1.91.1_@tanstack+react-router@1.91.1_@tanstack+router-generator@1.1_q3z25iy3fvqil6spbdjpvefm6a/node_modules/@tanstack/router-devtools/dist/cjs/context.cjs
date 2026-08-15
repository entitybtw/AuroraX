"use strict";
Object.defineProperty(exports, Symbol.toStringTag, { value: "Module" });
const React = require("react");
const ShadowDomTargetContext = React.createContext(void 0);
const DevtoolsOnCloseContext = React.createContext(void 0);
const useDevtoolsOnClose = () => {
  const context = React.useContext(DevtoolsOnCloseContext);
  if (!context) {
    throw new Error(
      "useDevtoolsOnClose must be used within a TanStackRouterDevtools component"
    );
  }
  return context;
};
exports.DevtoolsOnCloseContext = DevtoolsOnCloseContext;
exports.ShadowDomTargetContext = ShadowDomTargetContext;
exports.useDevtoolsOnClose = useDevtoolsOnClose;
//# sourceMappingURL=context.cjs.map
