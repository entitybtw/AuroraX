// Vite/Vitest type augmentation for the Aurora UI workspace.
/// <reference types="vite/client" />
/// <reference types="vitest" />
/// <reference types="react" />
/// <reference types="react-dom" />

import type { JSX as ReactJSX } from "react";

declare global {
  namespace JSX {
    type Element = ReactJSX.Element;
  }

  interface Window {
    /**
     * Base URL prefix the gateway is mounted under (e.g. "/" or "/g"). Set
     * once at boot from the <meta name="aurora-base-path"> tag injected by
     * the Go dashboard handler. Read by lib/basepath.ts.
     */
    AURORA_BASE_PATH?: string;
  }
}



