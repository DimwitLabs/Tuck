import { useMemo, useState } from "react";

import { buildInstaller } from "../ssh/installer";
import type { VaultData } from "../vaultStore";
import { CopyButton, downloadBytes } from "./ui";

const FILE = "~/Downloads/tuck-install.sh";
const REVIEW = `less ${FILE}`;
const RUN = `sh ${FILE}`;

function maskKeys(script: string): string {
  return script.replace(
    /(-----BEGIN [A-Z ]*PRIVATE KEY-----\n)([\s\S]*?)(\n-----END [A-Z ]*PRIVATE KEY-----)/g,
    (_, begin: string, body: string, end: string) => `${begin}[${body.split("\n").length} lines of private key hidden]${end}`,
  );
}

const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? "" : "s"}`;

export function Install({ data }: { data: VaultData }) {
  const installer = useMemo(() => buildInstaller(data.hosts, data.credentials), [data.hosts, data.credentials]);
  const [viewing, setViewing] = useState(false);
  const [showKeys, setShowKeys] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const download = () => {
    setConfirming(false);
    downloadBytes(installer.script, "tuck-install.sh", "text/x-shellscript");
  };
  const shown = showKeys ? installer.script : maskKeys(installer.script);
  const lines = shown.replace(/\n$/, "").split("\n");

  return (
    <section className="panel">
      <div className="section-head">
        <h2>install on this machine</h2>
      </div>
      <p className="lede">
        one script puts <b>{plural(installer.keyCount, "key")}</b> and <b>{plural(installer.hostCount, "host")}</b> into{" "}
        <code>~/.ssh/tuck</code> and adds a single <code>Include</code> line to the top of <code>~/.ssh/config</code>,
        after backing it up. running it again replaces that folder, so this machine always matches the vault.{" "}
        <code>--remove</code> undoes all of it.
      </p>
      <ol className="steps">
        <li>
          <span>download it.</span>
          {confirming ? (
            <div className="secret-row">
              <span className="warn-text">
                it holds {plural(installer.keyCount, "private key")} in the clear until you run it. don't leave it lying around.
              </span>
              <button className="act go" onClick={download}>
                download it
              </button>
              <button className="act quiet" onClick={() => setConfirming(false)}>
                cancel
              </button>
            </div>
          ) : (
            <div className="secret-row">
              <button className="act go" onClick={() => (installer.keyCount > 0 ? setConfirming(true) : download())}>
                download tuck-install.sh
              </button>
            </div>
          )}
        </li>
        <li>
          <span>read it before you run it, here or in your terminal.</span>
          <div className="secret-row">
            <button className="act" onClick={() => setViewing((v) => !v)} aria-expanded={viewing}>
              {viewing ? "hide the script" : "show the script"}
            </button>
            <code>{REVIEW}</code>
            <CopyButton text={REVIEW} />
          </div>
        </li>
        <li>
          <span>run it. it holds your private keys in the clear, so it deletes itself once they're installed.</span>
          <div className="secret-row">
            <code>{RUN}</code>
            <CopyButton text={RUN} />
          </div>
        </li>
      </ol>

      {viewing && (
        <div className="script-box">
          <div className="script-bar">
            <span className="stamp">
              tuck-install.sh · {installer.script.split("\n").length - 1} lines
            </span>
            <label className="check">
              <input type="checkbox" checked={showKeys} onChange={(e) => setShowKeys(e.target.checked)} />
              show private keys
            </label>
          </div>
          <pre className="script" aria-label="install script">
            {lines.map((line, i) => (
              <span key={i} className="script-line">
                {line}
                {"\n"}
              </span>
            ))}
          </pre>
        </div>
      )}

      {installer.warnings.length > 0 && (
        <div className="warning">
          <h3>left out of the script</h3>
          <ul>
            {installer.warnings.map((w) => (
              <li key={w}>{w}</li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}
