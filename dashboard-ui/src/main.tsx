import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { getBasePath } from "./lib/basepath";
import { initI18n } from "./lib/i18n";
import "./styles/globals.css";

// Snapshot the configured base path before any component renders so
// downstream lib/api fetches see a stable value.
getBasePath();

const container = document.getElementById("root");
if (!container) {
  throw new Error("aurora SPA: #root container missing from index.html");
}

// Boot sequence: the persisted UI locale (and its translation catalog) must be
// loaded BEFORE the app modules are imported — several modules translate
// constants at module scope (guide tables, feature rows, stat presets), and
// those evaluate t() exactly once at import time. A failed locale fetch (e.g.
// not signed in yet) silently falls back to the source language.
await initI18n();
const { App } = await import("./app");

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
