//#region \0rolldown/runtime.js
var __defProp = Object.defineProperty;
var __exportAll = (all, no_symbols) => {
	let target = {};
	for (var name in all) __defProp(target, name, {
		get: all[name],
		enumerable: true
	});
	if (!no_symbols) __defProp(target, Symbol.toStringTag, { value: "Module" });
	return target;
};
//#endregion
let seroval = require("seroval");
//#region web/abort-signal.ts
const PROMISE_TO_ABORT_SIGNAL = (promise) => {
	const controller = new AbortController();
	const abort = controller.abort.bind(controller);
	promise.then(abort, abort);
	return controller;
};
function resolveAbortSignalResult(resolve) {
	resolve(this.reason);
}
function resolveAbortSignal(resolve) {
	this.addEventListener("abort", resolveAbortSignalResult.bind(this, resolve), { once: true });
}
function abortSignalToPromise(signal) {
	return new Promise(resolveAbortSignal.bind(signal));
}
const ABORT_CONTROLLER = {};
const AbortSignalPlugin = /* @__PURE__ */ (0, seroval.createPlugin)({
	tag: "seroval-plugins/web/AbortSignal",
	extends: [/* @__PURE__ */ (0, seroval.createPlugin)({
		tag: "seroval-plugins/web/AbortControllerFactoryPlugin",
		test(value) {
			return value === ABORT_CONTROLLER;
		},
		parse: {
			sync() {
				return ABORT_CONTROLLER;
			},
			async async() {
				return await Promise.resolve(ABORT_CONTROLLER);
			},
			stream() {
				return ABORT_CONTROLLER;
			}
		},
		serialize() {
			return PROMISE_TO_ABORT_SIGNAL.toString();
		},
		deserialize() {
			return PROMISE_TO_ABORT_SIGNAL;
		}
	})],
	test(value) {
		if (typeof AbortSignal === "undefined") return false;
		return value instanceof AbortSignal;
	},
	parse: {
		sync(value, ctx) {
			if (value.aborted) return { reason: ctx.parse(value.reason) };
			return {};
		},
		async async(value, ctx) {
			if (value.aborted) return { reason: await ctx.parse(value.reason) };
			const result = await abortSignalToPromise(value);
			return { reason: await ctx.parse(result) };
		},
		stream(value, ctx) {
			if (value.aborted) return { reason: ctx.parse(value.reason) };
			const promise = abortSignalToPromise(value);
			return {
				factory: ctx.parse(ABORT_CONTROLLER),
				controller: ctx.parse(promise)
			};
		}
	},
	serialize(node, ctx) {
		if (node.reason) return "AbortSignal.abort(" + ctx.serialize(node.reason) + ")";
		if (node.controller && node.factory) return "(" + ctx.serialize(node.factory) + ")(" + ctx.serialize(node.controller) + ").signal";
		return "(new AbortController).signal";
	},
	deserialize(node, ctx) {
		if (node.reason) return AbortSignal.abort(ctx.deserialize(node.reason));
		if (node.controller) return PROMISE_TO_ABORT_SIGNAL(ctx.deserialize(node.controller)).signal;
		return new AbortController().signal;
	}
});
//#endregion
//#region web/blob.ts
const BlobPlugin = /* @__PURE__ */ (0, seroval.createPlugin)({
	tag: "seroval-plugins/web/Blob",
	test(value) {
		if (typeof Blob === "undefined") return false;
		return value instanceof Blob;
	},
	parse: { async async(value, ctx) {
		return {
			type: await ctx.parse(value.type),
			buffer: await ctx.parse(await value.arrayBuffer())
		};
	} },
	serialize(node, ctx) {
		return "new Blob([" + ctx.serialize(node.buffer) + "],{type:" + ctx.serialize(node.type) + "})";
	},
	deserialize(node, ctx) {
		return new Blob([ctx.deserialize(node.buffer)], { type: ctx.deserialize(node.type) });
	}
});
//#endregion
//#region web/custom-event.ts
function createCustomEventOptions(current) {
	return {
		detail: current.detail,
		bubbles: current.bubbles,
		cancelable: current.cancelable,
		composed: current.composed
	};
}
const CustomEventPlugin = /* @__PURE__ */ (0, seroval.createPlugin)({
	tag: "seroval-plugins/web/CustomEvent",
	test(value) {
		if (typeof CustomEvent === "undefined") return false;
		return value instanceof CustomEvent;
	},
	parse: {
		sync(value, ctx) {
			return {
				type: ctx.parse(value.type),
				options: ctx.parse(createCustomEventOptions(value))
			};
		},
		async async(value, ctx) {
			return {
				type: await ctx.parse(value.type),
				options: await ctx.parse(createCustomEventOptions(value))
			};
		},
		stream(value, ctx) {
			return {
				type: ctx.parse(value.type),
				options: ctx.parse(createCustomEventOptions(value))
			};
		}
	},
	serialize(node, ctx) {
		return "new CustomEvent(" + ctx.serialize(node.type) + "," + ctx.serialize(node.options) + ")";
	},
	deserialize(node, ctx) {
		return new CustomEvent(ctx.deserialize(node.type), ctx.deserialize(node.options));
	}
});
//#endregion
//#region web/dom-exception.ts
const DOMExceptionPlugin = /* @__PURE__ */ (0, seroval.createPlugin)({
	tag: "seroval-plugins/web/DOMException",
	test(value) {
		if (typeof DOMException === "undefined") return false;
		return value instanceof DOMException;
	},
	parse: {
		sync(value, ctx) {
			return {
				name: ctx.parse(value.name),
				message: ctx.parse(value.message)
			};
		},
		async async(value, ctx) {
			return {
				name: await ctx.parse(value.name),
				message: await ctx.parse(value.message)
			};
		},
		stream(value, ctx) {
			return {
				name: ctx.parse(value.name),
				message: ctx.parse(value.message)
			};
		}
	},
	serialize(node, ctx) {
		return "new DOMException(" + ctx.serialize(node.message) + "," + ctx.serialize(node.name) + ")";
	},
	deserialize(node, ctx) {
		return new DOMException(ctx.deserialize(node.message), ctx.deserialize(node.name));
	}
});
//#endregion
//#region web/event.ts
function createEventOptions(current) {
	return {
		bubbles: current.bubbles,
		cancelable: current.cancelable,
		composed: current.composed
	};
}
const EventPlugin = /* @__PURE__ */ (0, seroval.createPlugin)({
	tag: "seroval-plugins/web/Event",
	test(value) {
		if (typeof Event === "undefined") return false;
		return value instanceof Event;
	},
	parse: {
		sync(value, ctx) {
			return {
				type: ctx.parse(value.type),
				options: ctx.parse(createEventOptions(value))
			};
		},
		async async(value, ctx) {
			return {
				type: await ctx.parse(value.type),
				options: await ctx.parse(createEventOptions(value))
			};
		},
		stream(value, ctx) {
			return {
				type: ctx.parse(value.type),
				options: ctx.parse(createEventOptions(value))
			};
		}
	},
	serialize(node, ctx) {
		return "new Event(" + ctx.serialize(node.type) + "," + ctx.serialize(node.options) + ")";
	},
	deserialize(node, ctx) {
		return new Event(ctx.deserialize(node.type), ctx.deserialize(node.options));
	}
});
//#endregion
//#region web/file.ts
const FilePlugin = /* @__PURE__ */ (0, seroval.createPlugin)({
	tag: "seroval-plugins/web/File",
	test(value) {
		if (typeof File === "undefined") return false;
		return value instanceof File;
	},
	parse: { async async(value, ctx) {
		return {
			name: await ctx.parse(value.name),
			options: await ctx.parse({
				type: value.type,
				lastModified: value.lastModified
			}),
			buffer: await ctx.parse(await value.arrayBuffer())
		};
	} },
	serialize(node, ctx) {
		return "new File([" + ctx.serialize(node.buffer) + "]," + ctx.serialize(node.name) + "," + ctx.serialize(node.options) + ")";
	},
	deserialize(node, ctx) {
		return new File([ctx.deserialize(node.buffer)], ctx.deserialize(node.name), ctx.deserialize(node.options));
	}
});
//#endregion
//#region web/form-data.ts
function convertFormData(instance) {
	const items = [];
	instance.forEach((value, key) => {
		items.push([key, value]);
	});
	return items;
}
const FORM_DATA_FACTORY = {};
const FORM_DATA_FACTORY_CONSTRUCTOR = (e, f = new FormData(), i = 0, s = e.length, t) => {
	for (; i < s; i++) {
		t = e[i];
		f.append(t[0], t[1]);
	}
	return f;
};
const FormDataPlugin = /* @__PURE__ */ (0, seroval.createPlugin)({
	tag: "seroval-plugins/web/FormData",
	extends: [FilePlugin, /* @__PURE__ */ (0, seroval.createPlugin)({
		tag: "seroval-plugins/web/FormDataFactory",
		test(value) {
			return value === FORM_DATA_FACTORY;
		},
		parse: {
			sync() {
				return FORM_DATA_FACTORY;
			},
			async async() {
				return await Promise.resolve(FORM_DATA_FACTORY);
			},
			stream() {
				return FORM_DATA_FACTORY;
			}
		},
		serialize() {
			return FORM_DATA_FACTORY_CONSTRUCTOR.toString();
		},
		deserialize() {
			return FORM_DATA_FACTORY;
		}
	})],
	test(value) {
		if (typeof FormData === "undefined") return false;
		return value instanceof FormData;
	},
	parse: {
		sync(value, ctx) {
			return {
				factory: ctx.parse(FORM_DATA_FACTORY),
				entries: ctx.parse(convertFormData(value))
			};
		},
		async async(value, ctx) {
			return {
				factory: await ctx.parse(FORM_DATA_FACTORY),
				entries: await ctx.parse(convertFormData(value))
			};
		},
		stream(value, ctx) {
			return {
				factory: ctx.parse(FORM_DATA_FACTORY),
				entries: ctx.parse(convertFormData(value))
			};
		}
	},
	serialize(node, ctx) {
		return "(" + ctx.serialize(node.factory) + ")(" + ctx.serialize(node.entries) + ")";
	},
	deserialize(node, ctx) {
		return FORM_DATA_FACTORY_CONSTRUCTOR(ctx.deserialize(node.entries));
	}
});
//#endregion
//#region web/headers.ts
function convertHeaders(instance) {
	const items = [];
	instance.forEach((value, key) => {
		items.push([key, value]);
	});
	return items;
}
const HeadersPlugin = /* @__PURE__ */ (0, seroval.createPlugin)({
	tag: "seroval-plugins/web/Headers",
	test(value) {
		if (typeof Headers === "undefined") return false;
		return value instanceof Headers;
	},
	parse: {
		sync(value, ctx) {
			return { value: ctx.parse(convertHeaders(value)) };
		},
		async async(value, ctx) {
			return { value: await ctx.parse(convertHeaders(value)) };
		},
		stream(value, ctx) {
			return { value: ctx.parse(convertHeaders(value)) };
		}
	},
	serialize(node, ctx) {
		return "new Headers(" + ctx.serialize(node.value) + ")";
	},
	deserialize(node, ctx) {
		return new Headers(ctx.deserialize(node.value));
	}
});
//#endregion
//#region web/image-data.ts
const ImageDataPlugin = /* @__PURE__ */ (0, seroval.createPlugin)({
	tag: "seroval-plugins/web/ImageData",
	test(value) {
		if (typeof ImageData === "undefined") return false;
		return value instanceof ImageData;
	},
	parse: {
		sync(value, ctx) {
			return {
				data: ctx.parse(value.data),
				width: ctx.parse(value.width),
				height: ctx.parse(value.height),
				options: ctx.parse({ colorSpace: value.colorSpace })
			};
		},
		async async(value, ctx) {
			return {
				data: await ctx.parse(value.data),
				width: await ctx.parse(value.width),
				height: await ctx.parse(value.height),
				options: await ctx.parse({ colorSpace: value.colorSpace })
			};
		},
		stream(value, ctx) {
			return {
				data: ctx.parse(value.data),
				width: ctx.parse(value.width),
				height: ctx.parse(value.height),
				options: ctx.parse({ colorSpace: value.colorSpace })
			};
		}
	},
	serialize(node, ctx) {
		return "new ImageData(" + ctx.serialize(node.data) + "," + ctx.serialize(node.width) + "," + ctx.serialize(node.height) + "," + ctx.serialize(node.options) + ")";
	},
	deserialize(node, ctx) {
		return new ImageData(ctx.deserialize(node.data), ctx.deserialize(node.width), ctx.deserialize(node.height), ctx.deserialize(node.options));
	}
});
//#endregion
//#region web/readable-stream.ts
const READABLE_STREAM_FACTORY = {};
const READABLE_STREAM_FACTORY_CONSTRUCTOR = (stream) => new ReadableStream({ start(controller) {
	stream.on({
		next(value) {
			try {
				controller.enqueue(value);
			} catch (_error) {}
		},
		throw(value) {
			controller.error(value);
		},
		return() {
			try {
				controller.close();
			} catch (_error) {}
		}
	});
} });
const ReadableStreamFactoryPlugin = /* @__PURE__ */ (0, seroval.createPlugin)({
	tag: "seroval-plugins/web/ReadableStreamFactory",
	test(value) {
		return value === READABLE_STREAM_FACTORY;
	},
	parse: {
		sync() {
			return READABLE_STREAM_FACTORY;
		},
		async async() {
			return await Promise.resolve(READABLE_STREAM_FACTORY);
		},
		stream() {
			return READABLE_STREAM_FACTORY;
		}
	},
	serialize() {
		return READABLE_STREAM_FACTORY_CONSTRUCTOR.toString();
	},
	deserialize() {
		return READABLE_STREAM_FACTORY;
	}
});
async function drainStream(stream, reader) {
	try {
		const result = await reader.read();
		if (result.done) {
			stream.return(result.value);
			reader.releaseLock();
		} else {
			stream.next(result.value);
			await drainStream(stream, reader);
		}
	} catch (error) {
		stream.throw(error);
	}
}
function cleanupStream(reader) {
	reader.cancel().catch(() => {});
	reader.releaseLock();
}
function toStream(value) {
	const stream = (0, seroval.createStream)();
	const reader = value.getReader();
	const cleanup = cleanupStream.bind(null, reader);
	drainStream(stream, reader).catch(cleanup);
	return [stream, cleanup];
}
const ReadableStreamPlugin = /* @__PURE__ */ (0, seroval.createPlugin)({
	tag: "seroval/plugins/web/ReadableStream",
	extends: [ReadableStreamFactoryPlugin],
	test(value) {
		if (typeof ReadableStream === "undefined") return false;
		return value instanceof ReadableStream;
	},
	parse: {
		sync(_value, ctx) {
			return {
				factory: ctx.parse(READABLE_STREAM_FACTORY),
				stream: ctx.parse((0, seroval.createStream)())
			};
		},
		async async(value, ctx) {
			return {
				factory: await ctx.parse(READABLE_STREAM_FACTORY),
				stream: await ctx.parse(toStream(value)[0])
			};
		},
		stream(value, ctx) {
			const [stream, cleanup] = toStream(value);
			ctx.addCleanup(cleanup);
			return {
				factory: ctx.parse(READABLE_STREAM_FACTORY),
				stream: ctx.parse(stream)
			};
		}
	},
	serialize(node, ctx) {
		return "(" + ctx.serialize(node.factory) + ")(" + ctx.serialize(node.stream) + ")";
	},
	deserialize(node, ctx) {
		const stream = ctx.deserialize(node.stream);
		return READABLE_STREAM_FACTORY_CONSTRUCTOR(stream);
	}
});
//#endregion
//#region web/request.ts
function createRequestOptions(current, body) {
	return {
		body,
		cache: current.cache,
		credentials: current.credentials,
		headers: current.headers,
		integrity: current.integrity,
		keepalive: current.keepalive,
		method: current.method,
		mode: current.mode,
		redirect: current.redirect,
		referrer: current.referrer,
		referrerPolicy: current.referrerPolicy
	};
}
const RequestPlugin = /* @__PURE__ */ (0, seroval.createPlugin)({
	tag: "seroval-plugins/web/Request",
	extends: [ReadableStreamPlugin, HeadersPlugin],
	test(value) {
		if (typeof Request === "undefined") return false;
		return value instanceof Request;
	},
	parse: {
		async async(value, ctx) {
			return {
				url: await ctx.parse(value.url),
				options: await ctx.parse(createRequestOptions(value, value.body && !value.bodyUsed ? await value.clone().arrayBuffer() : null))
			};
		},
		stream(value, ctx) {
			return {
				url: ctx.parse(value.url),
				options: ctx.parse(createRequestOptions(value, value.body && !value.bodyUsed ? value.clone().body : null))
			};
		}
	},
	serialize(node, ctx) {
		return "new Request(" + ctx.serialize(node.url) + "," + ctx.serialize(node.options) + ")";
	},
	deserialize(node, ctx) {
		return new Request(ctx.deserialize(node.url), ctx.deserialize(node.options));
	}
});
//#endregion
//#region web/response.ts
function createResponseOptions(current) {
	return {
		headers: current.headers,
		status: current.status,
		statusText: current.statusText
	};
}
const ResponsePlugin = /* @__PURE__ */ (0, seroval.createPlugin)({
	tag: "seroval-plugins/web/Response",
	extends: [ReadableStreamPlugin, HeadersPlugin],
	test(value) {
		if (typeof Response === "undefined") return false;
		return value instanceof Response;
	},
	parse: {
		async async(value, ctx) {
			return {
				body: await ctx.parse(value.body && !value.bodyUsed ? await value.clone().arrayBuffer() : null),
				options: await ctx.parse(createResponseOptions(value))
			};
		},
		stream(value, ctx) {
			return {
				body: ctx.parse(value.body && !value.bodyUsed ? value.clone().body : null),
				options: ctx.parse(createResponseOptions(value))
			};
		}
	},
	serialize(node, ctx) {
		return "new Response(" + ctx.serialize(node.body) + "," + ctx.serialize(node.options) + ")";
	},
	deserialize(node, ctx) {
		return new Response(ctx.deserialize(node.body), ctx.deserialize(node.options));
	}
});
//#endregion
//#region web/url.ts
const URLPlugin = /* @__PURE__ */ (0, seroval.createPlugin)({
	tag: "seroval-plugins/web/URL",
	test(value) {
		if (typeof URL === "undefined") return false;
		return value instanceof URL;
	},
	parse: {
		sync(value, ctx) {
			return { value: ctx.parse(value.href) };
		},
		async async(value, ctx) {
			return { value: await ctx.parse(value.href) };
		},
		stream(value, ctx) {
			return { value: ctx.parse(value.href) };
		}
	},
	serialize(node, ctx) {
		return "new URL(" + ctx.serialize(node.value) + ")";
	},
	deserialize(node, ctx) {
		return new URL(ctx.deserialize(node.value));
	}
});
//#endregion
//#region web/url-search-params.ts
const URLSearchParamsPlugin = /* @__PURE__ */ (0, seroval.createPlugin)({
	tag: "seroval-plugins/web/URLSearchParams",
	test(value) {
		if (typeof URLSearchParams === "undefined") return false;
		return value instanceof URLSearchParams;
	},
	parse: {
		sync(value, ctx) {
			return { value: ctx.parse(value.toString()) };
		},
		async async(value, ctx) {
			return { value: await ctx.parse(value.toString()) };
		},
		stream(value, ctx) {
			return { value: ctx.parse(value.toString()) };
		}
	},
	serialize(node, ctx) {
		return "new URLSearchParams(" + ctx.serialize(node.value) + ")";
	},
	deserialize(node, ctx) {
		return new URLSearchParams(ctx.deserialize(node.value));
	}
});
//#endregion
//#region web/index.ts
var web_exports = /* @__PURE__ */ __exportAll({
	AbortSignalPlugin: () => AbortSignalPlugin,
	BlobPlugin: () => BlobPlugin,
	CustomEventPlugin: () => CustomEventPlugin,
	DOMExceptionPlugin: () => DOMExceptionPlugin,
	EventPlugin: () => EventPlugin,
	FilePlugin: () => FilePlugin,
	FormDataPlugin: () => FormDataPlugin,
	HeadersPlugin: () => HeadersPlugin,
	ImageDataPlugin: () => ImageDataPlugin,
	ReadableStreamPlugin: () => ReadableStreamPlugin,
	RequestPlugin: () => RequestPlugin,
	ResponsePlugin: () => ResponsePlugin,
	URLPlugin: () => URLPlugin,
	URLSearchParamsPlugin: () => URLSearchParamsPlugin
});
//#endregion
Object.defineProperty(exports, "AbortSignalPlugin", {
	enumerable: true,
	get: function() {
		return AbortSignalPlugin;
	}
});
Object.defineProperty(exports, "BlobPlugin", {
	enumerable: true,
	get: function() {
		return BlobPlugin;
	}
});
Object.defineProperty(exports, "CustomEventPlugin", {
	enumerable: true,
	get: function() {
		return CustomEventPlugin;
	}
});
Object.defineProperty(exports, "DOMExceptionPlugin", {
	enumerable: true,
	get: function() {
		return DOMExceptionPlugin;
	}
});
Object.defineProperty(exports, "EventPlugin", {
	enumerable: true,
	get: function() {
		return EventPlugin;
	}
});
Object.defineProperty(exports, "FilePlugin", {
	enumerable: true,
	get: function() {
		return FilePlugin;
	}
});
Object.defineProperty(exports, "FormDataPlugin", {
	enumerable: true,
	get: function() {
		return FormDataPlugin;
	}
});
Object.defineProperty(exports, "HeadersPlugin", {
	enumerable: true,
	get: function() {
		return HeadersPlugin;
	}
});
Object.defineProperty(exports, "ImageDataPlugin", {
	enumerable: true,
	get: function() {
		return ImageDataPlugin;
	}
});
Object.defineProperty(exports, "ReadableStreamPlugin", {
	enumerable: true,
	get: function() {
		return ReadableStreamPlugin;
	}
});
Object.defineProperty(exports, "RequestPlugin", {
	enumerable: true,
	get: function() {
		return RequestPlugin;
	}
});
Object.defineProperty(exports, "ResponsePlugin", {
	enumerable: true,
	get: function() {
		return ResponsePlugin;
	}
});
Object.defineProperty(exports, "URLPlugin", {
	enumerable: true,
	get: function() {
		return URLPlugin;
	}
});
Object.defineProperty(exports, "URLSearchParamsPlugin", {
	enumerable: true,
	get: function() {
		return URLSearchParamsPlugin;
	}
});
Object.defineProperty(exports, "web_exports", {
	enumerable: true,
	get: function() {
		return web_exports;
	}
});
