import { useRef, useState } from "react";

import type { VaultData } from "../vaultStore";
import { downloadBytes, ErrorNote, errorMessage, megabytes, Search, Spinner, Stamps } from "./ui";

const size = (n: number) => (n < 1024 ? `${n} b` : n < 1 << 20 ? `${(n / 1024).toFixed(1)} kb` : `${(n / (1 << 20)).toFixed(1)} mb`);

interface Props {
  data: VaultData;
  query: string;
  onQuery: (q: string) => void;
  maxFileBytes: number;
}

export function Files({ data, query, onQuery, maxFileBytes }: Props) {
  const picker = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const q = query.toLowerCase();
  const shown = data.files.filter(({ data: f }) => !q || f.name.toLowerCase().includes(q) || f.tags.some((t) => t.toLowerCase().includes(q)));

  const upload = async (file: File) => {
    if (file.size > maxFileBytes) return setError(`${file.name} is over the ${megabytes(maxFileBytes)} limit.`);
    setBusy("upload");
    setError(null);
    try {
      await data.saveFile({ name: file.name, tags: [], size: file.size, mime: file.type || "application/octet-stream" }, new Uint8Array(await file.arrayBuffer()));
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  };

  return (
    <section className="panel">
      <div className="section-head">
        <h2>files</h2>
        <button className="act go" onClick={() => picker.current?.click()} disabled={busy === "upload"}>
          {busy === "upload" ? <Spinner label="encrypting…" /> : "upload a file"}
        </button>
        <input
          ref={picker}
          type="file"
          hidden
          onChange={(e) => {
            const f = e.target.files?.[0];
            e.target.value = "";
            if (f) void upload(f);
          }}
        />
      </div>
      <ErrorNote error={error} />
      {data.files.length > 0 && <Search value={query} onChange={onQuery} what="files" />}
      {shown.length === 0 ? (
        <p className="empty">{query ? "nothing matches." : `certificates, .env files, recovery codes: anything up to ${megabytes(maxFileBytes)}, encrypted before it leaves this tab.`}</p>
      ) : (
        <ul className="register">
          {shown.map(({ id, data: f }) => (
            <li key={id} className="entry">
              <h3>{f.name}</h3>
              <p className="detail">
                <span>{size(f.size)}</span>
                <span>{f.mime}</span>
              </p>
              <Stamps items={f.tags} />
              <div className="ops">
                <button
                  className="act"
                  disabled={busy === id}
                  onClick={async () => {
                    setBusy(id);
                    try {
                      downloadBytes(await data.readFile(id), f.name, f.mime);
                    } catch (err) {
                      setError(errorMessage(err));
                    } finally {
                      setBusy(null);
                    }
                  }}
                >
                  {busy === id ? <Spinner label="decrypting…" /> : "download"}
                </button>
                <button
                  className="act danger"
                  onClick={async () => {
                    if (!confirm(`delete ${f.name}?`)) return;
                    try {
                      await data.remove(id);
                    } catch (err) {
                      setError(errorMessage(err));
                    }
                  }}
                >
                  delete
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
