import { useState } from "react";

import { exportVault } from "../export";
import type { VaultData } from "../vaultStore";
import { downloadBytes, ErrorNote, errorMessage, Spinner } from "./ui";

export function Export({ data }: { data: VaultData }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const download = async () => {
    if (!confirm("this file will hold every private key, password and file in plain text. anyone who gets it gets all of them. continue?")) return;
    setBusy(true);
    setError(null);
    try {
      const file = await exportVault(data.credentials, data.hosts, data.files, data.readFile);
      downloadBytes(JSON.stringify(file, null, 2), "tuck-export.json", "application/json");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="panel">
      <div className="section-head">
        <h2>export</h2>
      </div>
      <div className="warning">
        <p>
          <b>this file is not protected.</b> it holds every private key, password, host and file in the vault as plain
          json. anyone who has it has all of them.
        </p>
        <p>keep it offline, and delete it once you're done.</p>
      </div>
      <ErrorNote error={error} />
      <div className="actions">
        <button className="act danger" onClick={download} disabled={busy}>
          {busy ? <Spinner label="decrypting everything…" /> : "export decrypted json"}
        </button>
      </div>
    </section>
  );
}
