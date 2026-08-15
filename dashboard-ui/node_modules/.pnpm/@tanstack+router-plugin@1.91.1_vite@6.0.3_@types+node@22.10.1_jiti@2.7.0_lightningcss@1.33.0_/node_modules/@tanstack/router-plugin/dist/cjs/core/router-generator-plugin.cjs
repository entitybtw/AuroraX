"use strict";
var __create = Object.create;
var __defProp = Object.defineProperty;
var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
var __getOwnPropNames = Object.getOwnPropertyNames;
var __getProtoOf = Object.getPrototypeOf;
var __hasOwnProp = Object.prototype.hasOwnProperty;
var __copyProps = (to, from, except, desc) => {
  if (from && typeof from === "object" || typeof from === "function") {
    for (let key of __getOwnPropNames(from))
      if (!__hasOwnProp.call(to, key) && key !== except)
        __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
  }
  return to;
};
var __toESM = (mod, isNodeMode, target) => (target = mod != null ? __create(__getProtoOf(mod)) : {}, __copyProps(
  // If the importer is in node compatibility mode or this is not an ESM
  // file that has been converted to a CommonJS file using a Babel-
  // compatible transform (i.e. "__esModule" has not been set), then set
  // "default" to the CommonJS "module.exports" for node compatibility.
  isNodeMode || !mod || !mod.__esModule ? __defProp(target, "default", { value: mod, enumerable: true }) : target,
  mod
));
Object.defineProperty(exports, Symbol.toStringTag, { value: "Module" });
const node_path = require("node:path");
const routerGenerator = require("@tanstack/router-generator");
const config = require("./config.cjs");
let lock = false;
const checkLock = () => lock;
const setLock = (bool) => {
  lock = bool;
};
const PLUGIN_NAME = "unplugin:router-generator";
const unpluginRouterGeneratorFactory = (options = {}) => {
  let ROOT = process.cwd();
  let userConfig = options;
  const getRoutesDirectoryPath = () => {
    return node_path.isAbsolute(userConfig.routesDirectory) ? userConfig.routesDirectory : node_path.join(ROOT, userConfig.routesDirectory);
  };
  const generate = async () => {
    if (checkLock()) {
      return;
    }
    setLock(true);
    try {
      await routerGenerator.generator(userConfig, process.cwd());
    } catch (err) {
      console.error(err);
      console.info();
    } finally {
      setLock(false);
    }
  };
  const handleFile = async (file, event) => {
    const filePath = node_path.normalize(file);
    if (filePath === routerGenerator.resolveConfigPath({ configDirectory: ROOT })) {
      userConfig = config.getConfig(options, ROOT);
      return;
    }
    if (event === "update" && filePath === node_path.resolve(userConfig.generatedRouteTree)) {
      return;
    }
    const routesDirectoryPath = getRoutesDirectoryPath();
    if (filePath.startsWith(routesDirectoryPath)) {
      await generate();
    }
  };
  const run = async (cb) => {
    if (userConfig.enableRouteGeneration ?? true) {
      await cb();
    }
  };
  return {
    name: "router-generator-plugin",
    async watchChange(id, { event }) {
      await run(async () => {
        await handleFile(id, event);
      });
    },
    vite: {
      async configResolved(config$1) {
        ROOT = config$1.root;
        userConfig = config.getConfig(options, ROOT);
        await run(generate);
      }
    },
    async rspack(compiler) {
      userConfig = config.getConfig(options, ROOT);
      if (compiler.options.mode === "production") {
        compiler.hooks.beforeRun.tapPromise(PLUGIN_NAME, async () => {
          await run(generate);
        });
      } else {
        const routesDirectoryPath = getRoutesDirectoryPath();
        const chokidar = await import("chokidar");
        chokidar.watch(routesDirectoryPath, { ignoreInitial: true }).on("add", async () => {
          await run(generate);
        });
        let generated = false;
        compiler.hooks.watchRun.tapPromise(PLUGIN_NAME, async () => {
          if (!generated) {
            generated = true;
            return run(generate);
          }
        });
      }
    },
    async webpack(compiler) {
      userConfig = config.getConfig(options, ROOT);
      if (compiler.options.mode === "production") {
        compiler.hooks.beforeRun.tapPromise(PLUGIN_NAME, async () => {
          await run(generate);
        });
      } else {
        const routesDirectoryPath = getRoutesDirectoryPath();
        const chokidar = await import("chokidar");
        chokidar.watch(routesDirectoryPath, { ignoreInitial: true }).on("add", async () => {
          await run(generate);
        });
        let generated = false;
        compiler.hooks.watchRun.tapPromise(PLUGIN_NAME, async () => {
          if (!generated) {
            generated = true;
            return run(generate);
          }
        });
      }
      if (compiler.options.mode === "production") {
        compiler.hooks.done.tap(PLUGIN_NAME, (stats) => {
          console.info("✅ " + PLUGIN_NAME + ": route-tree generation done");
          setTimeout(() => {
            process.exit(0);
          });
        });
      }
    }
  };
};
exports.unpluginRouterGeneratorFactory = unpluginRouterGeneratorFactory;
//# sourceMappingURL=router-generator-plugin.cjs.map
