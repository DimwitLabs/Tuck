import { useEffect, useState } from "react";

import { api, type PasskeyRow } from "../api";
import { canEnrol, createPasskey, creationOptions, credentialJSON, deviceName, forgetDevice, passkeyProblem, rememberDevice } from "../passkey";
import { Board, ErrorNote, errorMessage, Spinner } from "./ui";

const when = (iso: string | null) => (iso ? new Date(iso).toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" }) : "not yet");

// Keeps a device's own list from offering a passkey this vault no longer knows.
const accepted = async (userId: string, rows: PasskeyRow[]) => {
  if (!userId || !PublicKeyCredential.signalAllAcceptedCredentials) return;
  try {
    await PublicKeyCredential.signalAllAcceptedCredentials({
      rpId: location.hostname,
      userId,
      allAcceptedCredentialIds: rows.map((r) => r.id),
    });
  } catch {
    // an older browser, or one that keeps no list of its own
  }
};

export function Door() {
  const [rows, setRows] = useState<PasskeyRow[] | null>(null);
  const [enabled, setEnabled] = useState(false);
  const [supported, setSupported] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [naming, setNaming] = useState<string | null>(null);

  useEffect(() => {
    void (async () => {
      setSupported(await canEnrol());
      try {
        const res = await api.passkeys();
        setRows(res.passkeys);
        setEnabled(res.enabled);
        void accepted(res.userId, res.passkeys);
      } catch (err) {
        setRows([]);
        setError(errorMessage(err));
      }
    })();
  }, []);

  const enrol = async () => {
    setBusy(true);
    setError(null);
    try {
      const options = creationOptions((await api.passkeyStart()) as never);
      const credential = await createPasskey(options);
      if (!credential) return;
      const known = new Set((rows ?? []).map((r) => r.id));
      const res = await api.passkeyFinish(deviceName(), credentialJSON(credential));
      rememberDevice();
      setRows(res.passkeys);
      setNaming(res.passkeys.find((r) => !known.has(r.id))?.id ?? null);
      void accepted(res.userId, res.passkeys);
    } catch (err) {
      const problem = err instanceof DOMException ? passkeyProblem(err) : errorMessage(err);
      if (problem) setError(problem);
    } finally {
      setBusy(false);
    }
  };

  const rename = async (row: PasskeyRow, label: string) => {
    setNaming(null);
    if (label.trim() === row.label) return;
    try {
      const res = await api.passkeyRename(row.id, label);
      setRows(res.passkeys);
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const forget = async (row: PasskeyRow) => {
    if (!confirm(`${row.label} will have to type the word again. forget it?`)) return;
    try {
      const res = await api.passkeyForget(row.id);
      if (res.passkeys.length === 0) forgetDevice();
      setRows(res.passkeys);
      void accepted(res.userId, res.passkeys);
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  return (
    <Board
      title="door"
      action={
        enabled && supported ? (
          <button type="button" className="act go" onClick={enrol} disabled={busy}>
            {busy ? <Spinner label="waiting for your fingerprint or face…" /> : "enrol this device"}
          </button>
        ) : undefined
      }
      intro={
        <>
          <p className="lede muted">
            the door is the word you type before tuck shows a login page. it hides tuck from everyone who doesn't know it, and it is not your password. enrol a
            device and that device opens the door with its own fingerprint, face or pin instead. logging in does not change, you still need your password and
            your three answers.
          </p>
          <ErrorNote error={error} />
        </>
      }
    >
      {rows === null ? (
        <p className="empty card">
          <Spinner label="asking tuck about this device…" />
        </p>
      ) : error && !enabled ? (
        <p className="empty card">couldn't reach tuck just now. try again.</p>
      ) : !enabled ? (
        <p className="empty card">off. tuck needs TUCK_ORIGIN set to the https address it answers on.</p>
      ) : !supported ? (
        <p className="empty card">this browser has no fingerprint or face to offer. the word still works.</p>
      ) : rows && rows.length > 0 ? (
        <ul className="register">
          {rows.map((row) => (
            <li key={row.id} className="entry">
              {naming === row.id ? (
                <form
                  className="naming"
                  onSubmit={(e) => {
                    e.preventDefault();
                    void rename(row, new FormData(e.currentTarget).get("label") as string);
                  }}
                >
                  <input name="label" aria-label="what to call this device" defaultValue={row.label} maxLength={40} autoFocus onBlur={(e) => void rename(row, e.target.value)} />
                </form>
              ) : (
                <h3>{row.label}</h3>
              )}
              <p className="detail">
                <span>enrolled {when(row.added)}</span>
                <span>last opened the door {when(row.lastUsed)}</span>
              </p>
              <div className="ops">
                <button type="button" className="act" onClick={() => setNaming(row.id)}>
                  rename
                </button>
                <button type="button" className="act danger" onClick={() => void forget(row)}>
                  forget
                </button>
              </div>
            </li>
          ))}
        </ul>
      ) : (
        <p className="empty card">no devices enrolled. every device still types the word.</p>
      )}
    </Board>
  );
}
