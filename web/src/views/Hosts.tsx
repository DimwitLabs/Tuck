import { useState, type FormEvent } from "react";

import { aliasesFor, renderConfig, slugify } from "../ssh/config";
import type { HostItem } from "../types";
import type { VaultData } from "../vaultStore";
import { CopyButton, ErrorNote, errorMessage, Field, Search, Sheet, splitList, Spinner, Stamps } from "./ui";

interface Props {
  data: VaultData;
  query: string;
  onQuery: (q: string) => void;
}

export function Hosts({ data, query, onQuery }: Props) {
  const [editing, setEditing] = useState<{ id?: string; data: HostItem } | null>(null);
  const credentialById = new Map(data.credentials.map((c) => [c.id, c.data]));
  const q = query.toLowerCase();
  const shown = data.hosts.filter(
    ({ data: h }) =>
      !q ||
      h.name.toLowerCase().includes(q) ||
      h.hostname.toLowerCase().includes(q) ||
      aliasesFor(h).some((a) => a.toLowerCase().includes(q)) ||
      h.tags.some((t) => t.toLowerCase().includes(q)),
  );
  const keysWithPrivate = data.credentials.filter((c) => c.data.privateKey).length;

  return (
    <section className="panel">
      <div className="section-head">
        <p className="lede">
          <b>{data.hosts.length}</b> host{data.hosts.length === 1 ? "" : "s"} and <b>{keysWithPrivate}</b> key
          {keysWithPrivate === 1 ? "" : "s"}, tucked away.
        </p>
        <button className="act go" onClick={() => setEditing({ data: { name: "", aliases: [], hostname: "", tags: [] } })}>
          new host
        </button>
      </div>
      {data.hosts.length > 0 && <Search value={query} onChange={onQuery} what="hosts" />}
      {shown.length === 0 ? (
        <p className="empty">{query ? "nothing matches." : "no hosts yet. each host becomes something you can type after ssh."}</p>
      ) : (
        <ul className="register">
          {shown.map(({ id, data: h }) => {
            const cred = h.credentialId ? credentialById.get(h.credentialId) : undefined;
            const user = h.user || cred?.username;
            const [alias, ...more] = aliasesFor(h);
            return (
              <li key={id} className="entry">
                <h3>{h.name}</h3>
                <p className="cmd">
                  ssh {alias}
                  {more.length > 0 && <span className="alt">or ssh {more.join(", ")}</span>}
                </p>
                <p className="detail">
                  <span>
                    {user ? `${user}@` : ""}
                    {h.hostname}
                    {h.port && h.port !== 22 ? `, port ${h.port}` : ""}
                  </span>
                  <span>{cred ? `key: ${cred.name}` : "no key, uses ssh-agent"}</span>
                  {h.proxyJump && <span>through {h.proxyJump}</span>}
                </p>
                <Stamps items={h.tags} />
                <div className="ops">
                  <CopyButton text={`ssh ${alias}`} />
                  <button className="act" onClick={() => setEditing({ id, data: h })}>
                    edit
                  </button>
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {editing && (
        <HostForm
          data={data}
          editingId={editing.id}
          initial={editing.data}
          onCancel={() => setEditing(null)}
          onSave={async (h) => {
            await data.save("host", h, editing.id);
            setEditing(null);
          }}
          onDelete={async () => {
            await data.remove(editing.id!);
            setEditing(null);
          }}
        />
      )}
    </section>
  );
}

function HostForm({
  data,
  editingId,
  initial,
  onSave,
  onCancel,
  onDelete,
}: {
  data: VaultData;
  editingId?: string;
  initial: HostItem;
  onSave: (h: HostItem) => Promise<void>;
  onCancel: () => void;
  onDelete: () => Promise<void>;
}) {
  const [name, setName] = useState(initial.name);
  const [aliases, setAliases] = useState(initial.aliases.join(" "));
  const [hostname, setHostname] = useState(initial.hostname);
  const [port, setPort] = useState(initial.port ? String(initial.port) : "");
  const [user, setUser] = useState(initial.user ?? "");
  const [credentialId, setCredentialId] = useState(initial.credentialId ?? "");
  const [proxyJump, setProxyJump] = useState(initial.proxyJump ?? "");
  const [tags, setTags] = useState(initial.tags.join(", "));
  const [options, setOptions] = useState(initial.options ?? "");
  const [notes, setNotes] = useState(initial.notes ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const draft: HostItem = {
    name: name.trim(),
    aliases: splitList(aliases),
    hostname: hostname.trim(),
    port: port ? Number(port) : undefined,
    user: user.trim() || undefined,
    credentialId: credentialId || undefined,
    proxyJump: proxyJump.trim() || undefined,
    tags: splitList(tags),
    options: options.trim() || undefined,
    notes: notes.trim() || undefined,
  };
  const others = data.hosts.filter((h) => h.id !== editingId);
  const preview = draft.name && draft.hostname ? renderConfig([{ id: "draft", data: draft }], data.credentials) : null;
  const chosen = data.credentials.find((c) => c.id === credentialId)?.data;
  const problem = preview?.warnings.find((w) => w.startsWith("skipped "))?.replace(/^skipped [^:]*: /, "") ?? null;
  const caution = preview?.warnings.find((w) => !w.startsWith("skipped ")) ?? null;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const taken = new Set(others.flatMap((h) => aliasesFor(h.data)));
    const clash = aliasesFor(draft).find((a) => taken.has(a));
    if (clash) return setError(`another host already answers to "${clash}".`);
    if (problem) return setError(problem);
    setBusy(true);
    setError(null);
    try {
      await onSave(draft);
    } catch (err) {
      setError(errorMessage(err));
      setBusy(false);
    }
  };

  return (
    <Sheet title={initial.name ? `edit ${initial.name}` : "new host"} onClose={onCancel}>
      <form className="stack" onSubmit={submit}>
        <div className="row">
          <Field label="name">
            <input className="line" value={name} onChange={(e) => setName(e.target.value)} placeholder="build server" autoFocus required />
          </Field>
          <Field label="aliases" hint={`what you type after ssh. defaults to ${slugify(name || "the-name")}`}>
            <input className="line" value={aliases} onChange={(e) => setAliases(e.target.value)} placeholder={slugify(name || "build server")} />
          </Field>
        </div>
        <div className="row">
          <Field label="hostname or ip">
            <input className="line" value={hostname} onChange={(e) => setHostname(e.target.value)} placeholder="build.example.com" required />
          </Field>
          <Field label="port">
            <input className="line" type="number" min={1} max={65535} value={port} onChange={(e) => setPort(e.target.value)} placeholder="22" />
          </Field>
        </div>
        <div className="row">
          <Field label="key">
            <select className="line" value={credentialId} onChange={(e) => setCredentialId(e.target.value)}>
              <option value="">none, use ssh-agent</option>
              {data.credentials.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.data.name}
                  {c.data.privateKey ? "" : " (no private key)"}
                </option>
              ))}
            </select>
          </Field>
          <Field label="user">
            <input className="line" value={user} onChange={(e) => setUser(e.target.value)} placeholder={chosen?.username ?? "root"} />
          </Field>
        </div>
        <div className="row">
          <Field label="jump host" hint="another alias to hop through">
            <input className="line" list="jump-hosts" value={proxyJump} onChange={(e) => setProxyJump(e.target.value)} />
            <datalist id="jump-hosts">
              {others.flatMap((h) => aliasesFor(h.data)).map((a) => (
                <option key={a} value={a} />
              ))}
            </datalist>
          </Field>
          <Field label="tags" hint="comma separated">
            <input className="line" value={tags} onChange={(e) => setTags(e.target.value)} />
          </Field>
        </div>
        <Field label="extra ssh_config options" hint="one per line, e.g. ServerAliveInterval 30">
          <textarea className="line mono" rows={2} value={options} onChange={(e) => setOptions(e.target.value)} />
        </Field>
        <Field label="notes">
          <textarea className="line" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} />
        </Field>
        {preview && (
          <Field label="what gets written">
            <pre className="preview">{preview.config.split("\n").slice(2).join("\n").trim() || "(skipped, see below)"}</pre>
          </Field>
        )}
        {caution && !problem && <p className="hint">{caution}</p>}
        <ErrorNote error={error ?? problem} />
        <div className="actions end">
          {editingId && (
            <button
              type="button"
              className="act danger"
              style={{ marginRight: "auto" }}
              onClick={async () => {
                if (!confirm(`delete ${initial.name}?`)) return;
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
            {busy ? <Spinner label="saving…" /> : "save"}
          </button>
        </div>
      </form>
    </Sheet>
  );
}
