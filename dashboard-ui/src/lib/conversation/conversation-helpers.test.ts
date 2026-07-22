import { describe, expect, it } from "vitest";
import {
  canShowConversation,
  escapeHTML,
  extractConversationErrorMessage,
  extractRequestPromptTextSegments,
  extractResponsesInputMessages,
  extractResponsesOutputText,
  extractText,
  hasConversationPayload,
  isConversationalPath,
  isConversationExcludedPath,
  looksLikeResponsesOutput,
  renderBodyWithConversationHighlights,
} from "./conversation-helpers";

describe("extractText", () => {
  it("handles strings, arrays, and objects", () => {
    expect(extractText(null)).toBe("");
    expect(extractText("  hi  ")).toBe("hi");
    expect(extractText(["a", { text: "b" }, { output_text: "c" }])).toBe("a\nb\nc");
    expect(extractText({ text: " hello " })).toBe("hello");
    expect(extractText({ a: 1 })).toContain('"a": 1');
    expect(extractText(42)).toBe("42");
  });
});

describe("extractResponsesInputMessages", () => {
  it("normalizes string and array forms to {role,text}", () => {
    expect(extractResponsesInputMessages("hi")).toEqual([{ role: "user", text: "hi" }]);
    expect(extractResponsesInputMessages([{ role: "ASSISTANT", content: "yo" }])).toEqual([
      { role: "assistant", text: "yo" },
    ]);
    expect(extractResponsesInputMessages([{ role: "user", content: "" }])).toEqual([]);
  });
});

describe("extractResponsesOutputText", () => {
  it("joins content[].text", () => {
    expect(
      extractResponsesOutputText({
        content: [{ text: "one" }, { text: "two" }, { type: "image" }],
      }),
    ).toBe("one\ntwo");
    expect(extractResponsesOutputText({ content: "fallback" })).toBe("fallback");
  });
});

describe("extractRequestPromptTextSegments", () => {
  it("collects instructions, messages, and input variants", () => {
    const segments = extractRequestPromptTextSegments({
      instructions: "sys",
      messages: [{ content: "hello" }, { content: [{ text: "world" }] }],
      input: [{ content: [{ output_text: "in1" }], text: "in2" }],
    });
    expect(segments).toEqual(["sys", "hello", "world", "in1", "in2"]);
  });

  it("returns [] for non-objects", () => {
    expect(extractRequestPromptTextSegments(null)).toEqual([]);
    expect(extractRequestPromptTextSegments("nope")).toEqual([]);
  });
});

describe("looksLikeResponsesOutput", () => {
  it("recognises message/assistant items", () => {
    expect(looksLikeResponsesOutput([{ type: "message" }])).toBe(true);
    expect(looksLikeResponsesOutput([{ role: "assistant" }])).toBe(true);
    expect(
      looksLikeResponsesOutput([{ content: [{ type: "output_text", text: "x" }] }]),
    ).toBe(true);
    expect(looksLikeResponsesOutput([])).toBe(false);
    expect(looksLikeResponsesOutput("nope")).toBe(false);
  });
});

describe("conversation path predicates", () => {
  it("identifies conversational paths", () => {
    expect(isConversationalPath("/v1/chat/completions")).toBe(true);
    expect(isConversationalPath("/v1/responses?stream=1")).toBe(true);
    expect(isConversationalPath("/v1/embeddings")).toBe(false);
    expect(isConversationalPath(null)).toBe(false);
  });

  it("excludes embeddings paths", () => {
    expect(isConversationExcludedPath("/v1/embeddings")).toBe(true);
    expect(isConversationExcludedPath("/v1/embeddings/foo")).toBe(true);
    expect(isConversationExcludedPath("/v1/chat/completions")).toBe(false);
  });
});

describe("hasConversationPayload + canShowConversation", () => {
  it("detects messages and choices in payloads", () => {
    expect(
      hasConversationPayload({
        data: { request_body: { messages: [{ role: "user", content: "hi" }] } },
      }),
    ).toBe(true);
    expect(
      hasConversationPayload({
        data: { response_body: { choices: [{}] } },
      }),
    ).toBe(true);
    expect(hasConversationPayload({ data: {} })).toBe(false);
  });

  it("respects excluded paths even with payloads", () => {
    expect(
      canShowConversation({
        path: "/v1/embeddings",
        data: { request_body: { messages: [{}] } },
      }),
    ).toBe(false);
    expect(
      canShowConversation({
        path: "/v1/chat/completions",
        data: { request_body: { messages: [{}] } },
      }),
    ).toBe(true);
  });
});

describe("extractConversationErrorMessage", () => {
  it("walks nested error fields", () => {
    expect(
      extractConversationErrorMessage({
        data: { response_body: { error: { message: "boom" } } },
      }),
    ).toBe("boom");
    expect(
      extractConversationErrorMessage({
        data: { error_message: '{"error":{"message":"nested"}}' },
      }),
    ).toBe("nested");
    expect(extractConversationErrorMessage(null)).toBe("");
  });
});

describe("escapeHTML + renderBodyWithConversationHighlights", () => {
  it("escapeHTML escapes the standard five entities", () => {
    expect(escapeHTML(`<a href="x">'&'</a>`)).toBe(
      "&lt;a href=&quot;x&quot;&gt;&#39;&amp;&#39;&lt;/a&gt;",
    );
  });

  it("returns escaped raw when conversation cannot be shown", () => {
    const html = renderBodyWithConversationHighlights(
      { path: "/v1/embeddings" },
      { foo: "<b>" },
      { formatJSON: (v) => JSON.stringify(v, null, 2) },
    );
    expect(html).toContain("&quot;foo&quot;");
    expect(html).toContain("&lt;b&gt;");
    expect(html).not.toContain("conversation-body-highlight");
  });

  it("wraps section keys in role-class spans for conversational paths", () => {
    const value = { messages: [{ role: "user", content: "hi" }] };
    const html = renderBodyWithConversationHighlights(
      { path: "/v1/chat/completions", data: { request_body: value } },
      value,
      { formatJSON: (v) => JSON.stringify(v, null, 2), canShowConversation: () => true },
    );
    expect(html).toContain('class="conversation-body-highlight conversation-user"');
    expect(html).toContain('data-conversation-trigger="1"');
  });

  it("returns 'Not captured' escaped when formatJSON yields it", () => {
    const html = renderBodyWithConversationHighlights(
      { path: "/v1/chat/completions" },
      null,
      { formatJSON: () => "Not captured" },
    );
    expect(html).toBe("Not captured");
  });
});
