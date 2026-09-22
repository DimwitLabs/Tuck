import type { PrismTheme } from "prism-react-renderer";

const tuck: PrismTheme = {
  plain: { color: "var(--ink)", backgroundColor: "var(--paper)" },
  styles: [
    { types: ["comment", "prolog", "doctype", "cdata"], style: { color: "var(--muted)", fontStyle: "italic" } },
    { types: ["function", "keyword", "builtin", "tag", "selector", "atrule"], style: { color: "var(--steel)" } },
    { types: ["string", "attr-value", "char", "url"], style: { color: "var(--straw)" } },
    { types: ["number", "boolean", "constant", "parameter", "operator", "punctuation"], style: { color: "var(--muted)" } },
    { types: ["property", "attr-name", "variable", "class-name"], style: { color: "var(--ink)" } },
  ],
};

export default tuck;
