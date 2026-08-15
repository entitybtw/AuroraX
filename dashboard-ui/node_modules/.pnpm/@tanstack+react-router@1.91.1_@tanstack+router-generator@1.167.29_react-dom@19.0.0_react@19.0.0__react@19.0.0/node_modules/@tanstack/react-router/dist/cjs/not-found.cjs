"use strict";
Object.defineProperty(exports, Symbol.toStringTag, { value: "Module" });
const jsxRuntime = require("react/jsx-runtime");
const CatchBoundary = require("./CatchBoundary.cjs");
const useRouterState = require("./useRouterState.cjs");
function notFound(options = {}) {
  options.isNotFound = true;
  if (options.throw) throw options;
  return options;
}
function isNotFound(obj) {
  return !!(obj == null ? void 0 : obj.isNotFound);
}
function CatchNotFound(props) {
  const resetKey = useRouterState.useRouterState({
    select: (s) => `not-found-${s.location.pathname}-${s.status}`
  });
  return /* @__PURE__ */ jsxRuntime.jsx(
    CatchBoundary.CatchBoundary,
    {
      getResetKey: () => resetKey,
      onCatch: (error, errorInfo) => {
        var _a;
        if (isNotFound(error)) {
          (_a = props.onCatch) == null ? void 0 : _a.call(props, error, errorInfo);
        } else {
          throw error;
        }
      },
      errorComponent: ({ error }) => {
        var _a;
        if (isNotFound(error)) {
          return (_a = props.fallback) == null ? void 0 : _a.call(props, error);
        } else {
          throw error;
        }
      },
      children: props.children
    }
  );
}
function DefaultGlobalNotFound() {
  return /* @__PURE__ */ jsxRuntime.jsx("p", { children: "Not Found" });
}
exports.CatchNotFound = CatchNotFound;
exports.DefaultGlobalNotFound = DefaultGlobalNotFound;
exports.isNotFound = isNotFound;
exports.notFound = notFound;
//# sourceMappingURL=not-found.cjs.map
