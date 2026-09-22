import type * as Preset from "@docusaurus/preset-classic";
import type { Config } from "@docusaurus/types";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import tuckPrism from "./src/prism-tuck";

const ORG = "DimwitLabs";
const NAME = "Tuck";
const SITE = "https://tuck.dimwit.me";
const DOCS = "https://docs.tuck.dimwit.me";
const REPO = `https://github.com/${ORG}/${NAME}`;
function releasedVersion() {
  try {
    const here = fileURLToPath(new URL(".", import.meta.url));
    return readFileSync(`${here}../CHANGELOG.md`, "utf8").match(/^## \[(\d+\.\d+\.\d+)\]/m)?.[1] ?? "";
  } catch {
    return "";
  }
}

const TAGLINE = "all your secrets, tucked in.";

const config: Config = {
  title: "tuck",
  tagline: TAGLINE,
  favicon: "img/favicon.svg",

  url: DOCS,
  baseUrl: "/",
  trailingSlash: false,

  organizationName: ORG,
  projectName: NAME,

  onBrokenLinks: "throw",
  onBrokenAnchors: "throw",

  markdown: {
    format: "mdx",
    hooks: { onBrokenMarkdownLinks: "throw" },
  },

  future: { v4: true },

  i18n: { defaultLocale: "en", locales: ["en"] },

  clientModules: ["./src/fonts.ts"],

  customFields: {
    appVersion: releasedVersion(),
    repo: REPO,
  },

  presets: [
    [
      "classic",
      {
        docs: {
          routeBasePath: "/",
          sidebarPath: "./sidebars.ts",
          editUrl: `${REPO}/tree/main/docs/`,
        },
        blog: false,
        theme: { customCss: ["./src/css/custom.css"] },
        sitemap: { lastmod: "date", changefreq: "weekly", ignorePatterns: ["/search"] },
      } satisfies Preset.Options,
    ],
  ],

  themes: [
    [
      "@easyops-cn/docusaurus-search-local",
      {
        docsRouteBasePath: "/",
        indexBlog: false,
        hashed: true,
        highlightSearchTermsOnTargetPage: true,
        searchResultLimits: 8,
        searchResultContextMaxLength: 60,
      },
    ],
  ],

  themeConfig: {
    image: `${SITE}/og.png`,
    metadata: [
      { name: "description", content: `Documentation for Tuck, a self-hosted vault for SSH keys, hosts and files. ${TAGLINE}` },
      { name: "theme-color", content: "#e9edf1", media: "(prefers-color-scheme: light)" },
      { name: "theme-color", content: "#0a0e13", media: "(prefers-color-scheme: dark)" },
      { name: "author", content: "Deepansh Khurana" },
      { name: "robots", content: "index, follow, max-image-preview:large" },
      { property: "og:site_name", content: "tuck" },
      { property: "og:type", content: "website" },
      { property: "og:image:width", content: "1200" },
      { property: "og:image:height", content: "630" },
      { property: "og:image:alt", content: "A cat tucked under a blue sheet, beside the words: all your secrets, tucked in." },
      { name: "twitter:image:alt", content: "A cat tucked under a blue sheet, beside the words: all your secrets, tucked in." },
    ],
    colorMode: { defaultMode: "light", disableSwitch: false, respectPrefersColorScheme: true },
    navbar: {
      title: "tuck",
      logo: { alt: "", src: "img/logo.svg", srcDark: "img/logo-dark.svg", href: SITE, target: "_self", height: 30, width: 30 },
      items: [
        { to: "/", label: "docs", position: "left", activeBaseRegex: "^/$" },
        { to: "/getting-started/run-it", label: "run it", position: "left" },
        { to: "/reference/configuration", label: "reference", position: "left", activeBasePath: "/reference" },
        { type: "search", position: "right" },
        { href: REPO, label: "github", position: "right" },
        { type: "custom-versionPill", position: "right" },
      ],
    },
    footer: {
      style: "light",
      links: [
        {
          title: "start",
          items: [
            { label: "what tuck is", to: "/" },
            { label: "run it", to: "/getting-started/run-it" },
            { label: "first login", to: "/getting-started/first-login" },
          ],
        },
        {
          title: "reference",
          items: [
            { label: "configuration", to: "/reference/configuration" },
            { label: "how it's encrypted", to: "/reference/how-it-is-encrypted" },
            { label: "what it protects against", to: "/reference/threat-model" },
          ],
        },
        {
          title: "project",
          items: [
            { label: "github", href: REPO },
            { label: "changelog", href: `${REPO}/blob/main/CHANGELOG.md` },
            { label: "report a vulnerability", href: `${REPO}/security/advisories/new` },
          ],
        },
      ],
      copyright:
        'made with love and labour, and backed by the <a href="https://dimwit.me/pledge">dimwit pledge</a>. mit licensed.',
    },
    docs: { sidebar: { hideable: false, autoCollapseCategories: false } },
    tableOfContents: { minHeadingLevel: 2, maxHeadingLevel: 3 },
    prism: {
      theme: tuckPrism,
      darkTheme: tuckPrism,
      additionalLanguages: ["bash", "nginx", "sql", "yaml", "ini"],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
