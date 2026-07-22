import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./app";
import { getBasePath } from "./lib/basepath";
import "./styles/globals.css";

// Snapshot the configured base path before any component renders so
// downstream lib/api fetches see a stable value.
getBasePath();

const container = document.getElementById("root");
if (!container) {
  throw new Error("aurora SPA: #root container missing from index.html");
}

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
