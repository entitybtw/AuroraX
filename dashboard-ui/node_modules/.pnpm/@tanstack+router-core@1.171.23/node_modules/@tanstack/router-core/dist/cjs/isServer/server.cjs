Object.defineProperty(exports, Symbol.toStringTag, { value: "Module" });
const require_load_server = require("../load-server.cjs");
//#region src/isServer/server.ts
const isServer = process.env.NODE_ENV === "test" ? void 0 : true;
//#endregion
exports.isServer = isServer;
exports.loadServerRoute = require_load_server.loadServerRoute;

//# sourceMappingURL=server.cjs.map