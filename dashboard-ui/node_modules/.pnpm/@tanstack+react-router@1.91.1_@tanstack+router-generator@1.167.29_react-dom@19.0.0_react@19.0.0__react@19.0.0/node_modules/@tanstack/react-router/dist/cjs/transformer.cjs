"use strict";
Object.defineProperty(exports, Symbol.toStringTag, { value: "Module" });
const utils = require("./utils.cjs");
const defaultTransformer = {
  stringify: (value) => JSON.stringify(value, function replacer(key, value2) {
    const keyVal = this[key];
    const transformer = transformers.find((t) => t.stringifyCondition(keyVal));
    if (transformer) {
      return transformer.stringify(keyVal);
    }
    return value2;
  }),
  parse: (value) => JSON.parse(value, function parser(key, value2) {
    const keyVal = this[key];
    const transformer = transformers.find((t) => t.parseCondition(keyVal));
    if (transformer) {
      return transformer.parse(keyVal);
    }
    return value2;
  })
};
const transformers = [
  {
    // Dates
    stringifyCondition: (value) => value instanceof Date,
    stringify: (value) => ({ $date: value.toISOString() }),
    parseCondition: (value) => utils.isPlainObject(value) && value.$date,
    parse: (value) => new Date(value.$date)
  },
  {
    // undefined
    stringifyCondition: (value) => value === void 0,
    stringify: () => ({ $undefined: "" }),
    parseCondition: (value) => utils.isPlainObject(value) && value.$undefined === "",
    parse: () => void 0
  }
];
exports.defaultTransformer = defaultTransformer;
//# sourceMappingURL=transformer.cjs.map
