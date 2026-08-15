class Store {
  constructor(initialState, options) {
    this.listeners = /* @__PURE__ */ new Set();
    this._batching = false;
    this._flushing = 0;
    this.subscribe = (listener) => {
      var _a, _b;
      this.listeners.add(listener);
      const unsub = (_b = (_a = this.options) == null ? void 0 : _a.onSubscribe) == null ? void 0 : _b.call(_a, listener, this);
      return () => {
        this.listeners.delete(listener);
        unsub == null ? void 0 : unsub();
      };
    };
    this.setState = (updater) => {
      var _a, _b, _c;
      const previous = this.state;
      this.state = ((_a = this.options) == null ? void 0 : _a.updateFn) ? this.options.updateFn(previous)(updater) : updater(previous);
      (_c = (_b = this.options) == null ? void 0 : _b.onUpdate) == null ? void 0 : _c.call(_b);
      this._flush();
    };
    this._flush = () => {
      if (this._batching) return;
      const flushId = ++this._flushing;
      this.listeners.forEach((listener) => {
        if (this._flushing !== flushId) return;
        listener();
      });
    };
    this.batch = (cb) => {
      if (this._batching) return cb();
      this._batching = true;
      cb();
      this._batching = false;
      this._flush();
    };
    this.state = initialState;
    this.options = options;
  }
}
export {
  Store
};
//# sourceMappingURL=index.js.map
