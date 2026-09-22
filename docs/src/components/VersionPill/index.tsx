import useDocusaurusContext from "@docusaurus/useDocusaurusContext";

export default function VersionPill({ mobile }: { mobile?: boolean }) {
  const { siteConfig } = useDocusaurusContext();
  const version = siteConfig.customFields?.appVersion as string;
  const repo = siteConfig.customFields?.repo as string;
  if (!version) return null;
  const pill = (
    <a className="version-pill" href={`${repo}/releases/tag/v${version}`} target="_blank" rel="noopener noreferrer" title={`tuck ${version}`}>
      v{version}
    </a>
  );
  return mobile ? <li className="menu__list-item version-pill-item">{pill}</li> : <div className="navbar__item version-pill-slot">{pill}</div>;
}
