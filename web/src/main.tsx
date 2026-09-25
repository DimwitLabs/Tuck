import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import "@fontsource/martian-mono/400.css";
import "@fontsource/martian-mono/500.css";
import "@fontsource/spectral/400.css";
import "@fontsource/spectral/400-italic.css";
import "@fontsource/spectral/500.css";
import "@fontsource/spectral/600.css";

import { App } from "./App";
import "./styles.css";
import { rememberTheme } from "./theme";

rememberTheme();

// Older versions left a key here naming tuck; a browser that holds one is telling anyone who looks that this address is a vault.
try {
  localStorage.removeItem("tuck.device");
} catch {
  // storage blocked: nothing was stored either
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
