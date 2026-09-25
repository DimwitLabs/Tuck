import { useState } from "react";

import type { Features } from "../api";
import type { Keyring, Vault } from "../crypto/vault";
import { type Loaded, useVaultData } from "../vaultStore";
import { Access } from "./Access";
import { Door } from "./Door";
import { Export } from "./Export";
import { Files } from "./Files";
import { How } from "./How";
import { Hosts } from "./Hosts";
import { Install } from "./Install";
import { Keys } from "./Keys";
import { Logo, ThemeSwitch } from "./ui";

const TABS = ["how", "hosts", "keys", "files", "install", "export", "access", "door"] as const;
type Tab = (typeof TABS)[number];

const HIDDEN: Record<Features, Tab[]> = { both: [], ssh: ["files"], files: ["hosts", "keys", "install"] };

interface Props {
  vault: Vault;
  keyring: Keyring;
  loaded: Loaded;
  username: string;
  questions: string[];
  features: Features;
  maxFileBytes: number;
  onRekeyed: (keyring: Keyring, questions: string[]) => void;
  onLock: () => void;
  onLogout: () => void;
}

export function VaultView({ vault, keyring, loaded, username, questions, features, maxFileBytes, onRekeyed, onLock, onLogout }: Props) {
  const data = useVaultData(vault, loaded);
  const tabs = TABS.filter((t) => !HIDDEN[features].includes(t));
  const [tab, setTab] = useState<Tab>(features === "files" ? "files" : "hosts");
  const [query, setQuery] = useState("");
  const counts: Partial<Record<Tab, number>> = {
    hosts: data.hosts.length,
    keys: data.credentials.length,
    files: data.files.length,
  };

  return (
    <div className="shell">
      <div className="deck">
        <div className="bar">
          <header className="appbar">
            <div className="appbar-main">
              <span className="wordmark">
                <Logo />
                tuck
              </span>
              <nav className="tabs" aria-label="sections">
                {tabs.map((t) => (
                  <button
                    key={t}
                    className={`act ${t === tab ? "here" : ""}`}
                    onClick={() => {
                      setTab(t);
                      setQuery("");
                    }}
                    aria-current={t === tab ? "page" : undefined}
                  >
                    {t}
                    {counts[t] !== undefined && <span className="count">{counts[t]}</span>}
                  </button>
                ))}
              </nav>
            </div>
            <div className="who">
              <button className="act" onClick={onLock} title="forget the keys in this tab; answer your questions to come back">
                lock
              </button>
              <button className="act quiet" onClick={onLogout}>
                log out
              </button>
            </div>
          </header>
          <ThemeSwitch />
        </div>
        {data.problems.length > 0 && (
          <div className="warning" role="alert">
            <h3>the server may have tampered with this vault</h3>
            <ul>
              {data.problems.map((p) => (
                <li key={p}>{p}</li>
              ))}
            </ul>
          </div>
        )}
        {tab === "how" && <How />}
        {tab === "hosts" && <Hosts data={data} query={query} onQuery={setQuery} />}
        {tab === "keys" && <Keys data={data} query={query} onQuery={setQuery} />}
        {tab === "files" && <Files data={data} query={query} onQuery={setQuery} maxFileBytes={maxFileBytes} />}
        {tab === "install" && <Install data={data} />}
        {tab === "export" && <Export data={data} />}
        {tab === "access" && <Access keyring={keyring} questions={questions} username={username} onRekeyed={onRekeyed} />}
        {tab === "door" && <Door />}
      </div>
    </div>
  );
}
