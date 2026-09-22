import type { SidebarsConfig } from "@docusaurus/plugin-content-docs";

const sidebars: SidebarsConfig = {
  docs: [
    "intro",
    {
      type: "category",
      label: "getting started",
      collapsed: false,
      items: ["getting-started/run-it", "getting-started/reverse-proxy", "getting-started/your-own-postgres", "getting-started/first-login"],
    },
    {
      type: "category",
      label: "using tuck",
      collapsed: false,
      items: ["using/unlocking", "using/keys-and-hosts", "using/installing-on-a-machine", "using/front-door", "using/backups"],
    },
    {
      type: "category",
      label: "reference",
      collapsed: false,
      items: [
        "reference/configuration",
        "reference/how-it-is-encrypted",
        "reference/threat-model",
        { type: "link", label: "changelog", href: "https://github.com/DimwitLabs/Tuck/blob/main/CHANGELOG.md" },
      ],
    },
  ],
};

export default sidebars;
