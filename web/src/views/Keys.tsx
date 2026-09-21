import { useState, type FormEvent } from "react";

import { slugify } from "../ssh/config";
import { generateKey, type KeySpec } from "../ssh/keygen";
import { describeKey } from "../ssh/openssh";
import type { CredentialItem } from "../types";
import type { VaultData } from "../vaultStore";
import { CopyButton, downloadBytes, ErrorNote, errorMessage, Field, nextFrame, Search, Sheet, splitList, Spinner, Stamps } from "./ui";

interface Props {
  data: VaultData;
  query: string;
  onQuery: (q: string) => void;
}

export function Keys({ data, query, onQuery }: Props) {
  const [editing, setEditing] = useState<{ id?: string; data: CredentialItem } | null>(null);
  const [revealed, setRevealed] = useState<string | null>(null);

  const q = query.toLowerCase();
  const shown = data.credentials.filter(
    ({ data: c }) =>
      !q || c.name.toLowerCase().includes(q) || c.tags.some((t) => t.toLowerCase().includes(q)) || c.username?.toLowerCase().includes(q),
  );
  const usedBy = (id: string) => data.hosts.filter((h) => h.data.credentialId === id).map((h) => h.data.name);
  const secret = revealed ? data.credentials.find((c) => c.id === revealed)?.data : undefined;

  return (
    <section className="panel">
      <div className="section-head">
        <h2>keys</h2>
        <button className="act go" onClick={() => setEditing({ data: { name: "", tags: [] } })}>
          new key
        </button>
      </div>
      {data.credentials.length > 0 && <Search value={query} onChange={onQuery} what="keys" />}
      {shown.length === 0 ? (
        <p className="empty">{query ? "nothing matches." : "no keys yet. generate one here, or paste an existing private key."}</p>
      ) : (
        <ul className="register">
          {shown.map(({ id, data: c }) => {
            const users = usedBy(id);
            return (
              <li key={id} className="entry">
                <h3>{c.name}</h3>
                <p className="detail">
                  {c.username && <span>{c.username}</span>}
                  <span>{users.length === 0 ? "not used by any host" : users.length === 1 ? `used by ${users[0]}` : `used by ${users.length} hosts`}</span>
                </p>
                {c.fingerprint && <p className="fp">{c.fingerprint}</p>}
                <Stamps items={[c.keyType, c.password && "password", c.passphrase && "passphrase", ...c.tags]} />
                <div className="ops">
                  {c.publicKey && <CopyButton text={c.publicKey} label="copy public key" />}
                  {(c.privateKey || c.password) && (
                    <button className="act" onClick={() => setRevealed(id)}>
                      reveal
                    </button>
                  )}
                  <button className="act" onClick={() => setEditing({ id, data: c })}>
                    edit
                  </button>
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {editing && (
        <CredentialForm
          initial={editing.data}
          usedBy={editing.id ? usedBy(editing.id) : []}
          onCancel={() => setEditing(null)}
          onSave={async (c) => {
            await data.save("credential", c, editing.id);
            setEditing(null);
          }}
          onDelete={
            editing.id
              ? async () => {
                  await data.remove(editing.id!);
                  setEditing(null);
                }
              : undefined
          }
        />
      )}

      {secret && (
        <Sheet title={secret.name} onClose={() => setRevealed(null)}>
          <div className="stack">
            {secret.password && (
              <Field label="password">
                <div className="secret-row">
                  <code>{secret.password}</code>
                  <CopyButton text={secret.password} secret />
                </div>
              </Field>
            )}
            {secret.passphrase && (
              <Field label="key passphrase">
                <div className="secret-row">
                  <code>{secret.passphrase}</code>
                  <CopyButton text={secret.passphrase} secret />
                </div>
              </Field>
            )}
            {secret.privateKey && (
              <Field label="private key">
                <pre className="secret">{secret.privateKey}</pre>
              </Field>
            )}
            {secret.privateKey && (
              <div className="actions">
                <CopyButton text={secret.privateKey} label="copy private key" secret />
                <button className="act" onClick={() => downloadBytes(secret.privateKey!, slugify(secret.name), "text/plain")}>
                  download private key
                </button>
                {secret.publicKey && (
                  <button className="act" onClick={() => downloadBytes(`${secret.publicKey}\n`, `${slugify(secret.name)}.pub`, "text/plain")}>
                    download public key
                  </button>
                )}
              </div>
            )}
            {secret.privateKey && (
              <p className="hint">
                ssh only accepts a private key file that only you can read: move it into <code>~/.ssh</code> and <code>chmod 600</code> it.
              </p>
            )}
          </div>
        </Sheet>
      )}
    </section>
  );
}

type KeyMode = "generate" | "paste" | "keep" | "none";

function CredentialForm({
  initial,
  usedBy,
  onSave,
  onCancel,
  onDelete,
}: {
  initial: CredentialItem;
  usedBy: string[];
  onSave: (c: CredentialItem) => Promise<void>;
  onCancel: () => void;
  onDelete?: () => Promise<void>;
}) {
  const hasKey = !!initial.privateKey;
  const [name, setName] = useState(initial.name);
  const [username, setUsername] = useState(initial.username ?? "");
  const [tags, setTags] = useState(initial.tags.join(", "));
  const [password, setPassword] = useState(initial.password ?? "");
  const [showPassword, setShowPassword] = useState(false);
  const [mode, setMode] = useState<KeyMode>(hasKey ? "keep" : initial.name ? "none" : "generate");
  const [spec, setSpec] = useState<KeySpec>("ed25519");
  const [privateKey, setPrivateKey] = useState("");
  const [publicKey, setPublicKey] = useState("");
  const [passphrase, setPassphrase] = useState(initial.passphrase ?? "");
  const [notes, setNotes] = useState(initial.notes ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    await nextFrame();
    try {
      const base: CredentialItem = {
        name: name.trim(),
        tags: splitList(tags),
        username: username.trim() || undefined,
        password: password || undefined,
        notes: notes.trim() || undefined,
      };
      let key: Partial<CredentialItem> = {};
      if (mode === "keep") {
        key = {
          privateKey: initial.privateKey,
          publicKey: initial.publicKey,
          fingerprint: initial.fingerprint,
          keyType: initial.keyType,
          passphrase: passphrase || undefined,
        };
      } else if (mode === "generate") {
        key = await generateKey(spec, `${slugify(name)}@tuck`);
      } else if (mode === "paste") {
        const pem = privateKey.trim() ? `${privateKey.trim()}\n` : undefined;
        key = { privateKey: pem, ...(await describeKey(pem, publicKey)), passphrase: passphrase || undefined };
      }
      if (mode === "none" && !base.password) throw new Error("add a key, a password, or both.");
      await onSave({ ...base, ...key });
    } catch (err) {
      setError(errorMessage(err));
      setBusy(false);
    }
  };

  const modes: [KeyMode, string][] = [
    ...(hasKey ? ([["keep", "keep current"]] as [KeyMode, string][]) : []),
    ["generate", "generate"],
    ["paste", "paste existing"],
    ["none", "no key"],
  ];
  const specs: [KeySpec, string][] = [
    ["ed25519", "ed25519"],
    ["rsa-3072", "rsa 3072"],
    ["rsa-4096", "rsa 4096"],
  ];

  return (
    <Sheet title={initial.name ? `edit ${initial.name}` : "new key"} onClose={onCancel}>
      <form className="stack" style={{ gap: "1.6rem" }} onSubmit={submit}>
        <Field label="name">
          <input className="line" value={name} onChange={(e) => setName(e.target.value)} placeholder="build server" autoFocus required />
        </Field>
        <div className="row">
          <Field label="username" hint="used by hosts that don't set their own">
            <input className="line" value={username} onChange={(e) => setUsername(e.target.value)} placeholder="alex" />
          </Field>
          <Field label="tags" hint="comma separated">
            <input className="line" value={tags} onChange={(e) => setTags(e.target.value)} placeholder="homelab, physical" />
          </Field>
        </div>

        <div className="stack" style={{ gap: ".6rem" }}>
          <span className="stamp">key</span>
          <div className="choice" role="radiogroup" aria-label="key">
            {modes.map(([m, label]) => (
              <button key={m} type="button" role="radio" aria-checked={mode === m} className={`act ${mode === m ? "here" : ""}`} onClick={() => setMode(m)}>
                {label}
              </button>
            ))}
          </div>
          {mode === "keep" && initial.fingerprint && <p className="fp">{initial.fingerprint}</p>}
          {mode === "generate" && (
            <>
              <div className="choice" role="radiogroup" aria-label="key type">
                {specs.map(([s, label]) => (
                  <button key={s} type="button" role="radio" aria-checked={spec === s} className={`act ${spec === s ? "here" : ""}`} onClick={() => setSpec(s)}>
                    {label}
                  </button>
                ))}
              </div>
              <p className="hint">ed25519 is short, fast and the right default. rsa is for old servers that insist.</p>
            </>
          )}
        </div>

        {mode === "paste" && (
          <>
            <Field label="private key" hint="openssh, pem or pkcs#8. the public key is worked out from it.">
              <textarea className="line mono" rows={5} value={privateKey} onChange={(e) => setPrivateKey(e.target.value)} placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" required />
            </Field>
            <Field label="public key" hint="only needed for passphrase-protected pem keys">
              <input className="line mono" value={publicKey} onChange={(e) => setPublicKey(e.target.value)} placeholder="ssh-ed25519 AAAA…" />
            </Field>
          </>
        )}
        {(mode === "paste" || mode === "keep") && (
          <Field label="key passphrase" hint="only if the key file itself is passphrase-protected">
            <input className="line" type="password" value={passphrase} onChange={(e) => setPassphrase(e.target.value)} autoComplete="off" />
          </Field>
        )}

        <Field label="password">
          <div className="secret-row">
            <input
              className="line"
              type={showPassword ? "text" : "password"}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="optional, for logins that use one"
              autoComplete="off"
            />
            <button type="button" className="act quiet" onClick={() => setShowPassword((v) => !v)}>
              {showPassword ? "hide" : "show"}
            </button>
          </div>
        </Field>
        <Field label="notes">
          <textarea className="line" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="where this key is authorised" />
        </Field>
        <ErrorNote error={error} />
        <div className="actions end">
          {onDelete && (
            <button
              type="button"
              className="act danger"
              style={{ marginRight: "auto" }}
              onClick={async () => {
                if (usedBy.length) return setError(`${usedBy.join(", ")} still use${usedBy.length === 1 ? "s" : ""} this key. pick another key for ${usedBy.length === 1 ? "it" : "them"} first.`);
                if (!confirm(`delete ${initial.name}? its private key is gone for good unless you have another copy.`)) return;
                try {
                  await onDelete();
                } catch (err) {
                  setError(errorMessage(err));
                }
              }}
            >
              delete
            </button>
          )}
          <button type="button" className="act quiet" onClick={onCancel}>
            cancel
          </button>
          <button className="act go" disabled={busy}>
            {busy ? <Spinner label={mode === "generate" ? "generating…" : "saving…"} /> : mode === "generate" ? "generate and save" : "save"}
          </button>
        </div>
      </form>
    </Sheet>
  );
}
