import { loadServerRoute } from "../load-server.js";
//#region src/isServer/server.ts
const isServer = process.env.NODE_ENV === "test" ? void 0 : true;
//#endregion
export { isServer, loadServerRoute };

//# sourceMappingURL=server.js.map