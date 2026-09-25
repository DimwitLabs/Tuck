import { useCallback, useEffect, useRef, useState } from "react";

import { api, type OpenStatus, type StepResponse } from "./api";
import { forgetClipboard } from "./clipboard";
import { randomBytes } from "./crypto/encoding";
import { enroll, keyringOf, type Keyring, Unlocker, Vault } from "./crypto/vault";
import { type Loaded, loadItems } from "./vaultStore";
import { Auth } from "./views/Auth";
import { Gate } from "./views/Gate";
import { Spinner } from "./views/ui";
import { Unlock } from "./views/Unlock";
import { VaultView } from "./views/VaultView";

const IDLE_LOCK_MS = 2 * 60 * 1000;
const IDLE = "locked after 2 minutes without activity.";
const IDLE_UNLOCK = "logged out after 2 minutes without an answer.";
const LEFT_TAB = "you left the tab, so tuck logged you out.";
const RELOADED = "reloading logs you out. your keys live in the tab, never on the server.";

const WAS_IN = "tuck.signed-in";

const mark = (on: boolean) => {
  try {
    if (on) sessionStorage.setItem(WAS_IN, "1");
    else sessionStorage.removeItem(WAS_IN);
  } catch {
    // a browser with storage blocked simply gets no hint
  }
};

const wasSignedIn = () => {
  try {
    return sessionStorage.getItem(WAS_IN) === "1";
  } catch {
    return false;
  }
};

type Phase =
  | { name: "loading" }
  | { name: "gate"; word: string; notice: string | null }
  | { name: "auth"; notice: string | null }
  | { name: "unlock"; unlocker: Unlocker; first: StepResponse }
  | { name: "vault"; vault: Vault; keyring: Keyring; loaded: Loaded; questions: string[] };

export function App() {
  const [status, setStatus] = useState<OpenStatus | null>(null);
  const [username, setUsername] = useState("");
  const [phase, setPhase] = useState<Phase>({ name: "loading" });
  const era = useRef(0);

  const refresh = useCallback(async (notice: string | null) => {
    try {
      const s = await api.status();
      if (s.gate) return setPhase({ name: "gate", word: s.word || "tuck.", notice });
      setStatus(s);
      setUsername((u) => s.session?.username ?? u);
      setPhase({ name: "auth", notice });
    } catch {
      setPhase({ name: "auth", notice: "tuck's server isn't answering. try again shortly." });
    }
  }, []);

  useEffect(() => {
    const reloaded = wasSignedIn();
    mark(false);
    void api
      .logout()
      .catch(() => {})
      .finally(() => refresh(reloaded ? RELOADED : null));
  }, [refresh]);

  const login = async (name: string, password: string) => {
    const pre = await api.prelogin(name.trim().toLowerCase());
    const started = era.current;
    const { unlocker, authKey } = await Unlocker.start(password, pre.kdfSalt, pre.kdf);
    const res = await api.login(name, authKey);
    const first = await api.unlockStart();
    if (era.current !== started) return;
    setUsername(res.username);
    setPhase({ name: "unlock", unlocker, first });
  };

  const signup = async (name: string, password: string, questions: string[], answers: string[]) => {
    const vaultKey = randomBytes(32);
    const enrollment = await enroll(password, questions, answers, vaultKey);
    const res = await api.signup(name, enrollment);
    setUsername(res.username);
    setStatus((s) => (s ? { ...s, signupOpen: false } : s));
    const vault = await Vault.fromRaw(vaultKey);
    const loaded: Loaded = { items: [], state: { manifest: { counter: 0, items: {} }, hash: "" }, problems: [] };
    setPhase({ name: "vault", vault, keyring: keyringOf(enrollment), loaded, questions: questions.map((q) => q.trim()) });
  };

  const unlocked = async (vault: Vault, keyring: Keyring, questions: string[]) => {
    const started = era.current;
    const loaded = await loadItems(vault);
    if (era.current !== started) return;
    setPhase({ name: "vault", vault, keyring, loaded, questions });
  };

  const rekeyed = useCallback((keyring: Keyring, questions: string[]) => {
    setPhase((p) => (p.name === "vault" ? { ...p, keyring, questions } : p));
  }, []);

  const lock = useCallback((notice: string | null) => {
    if (!notice) mark(false);
    era.current++;
    forgetClipboard();
    void api.lock().catch(() => {});
    setPhase({ name: "auth", notice });
  }, []);

  const logout = useCallback(
    (notice: string | null) => {
      if (!notice) mark(false);
      era.current++;
      forgetClipboard();
      setPhase({ name: "loading" });
      void api
        .logout()
        .catch(() => {})
        .finally(() => refresh(notice));
    },
    [refresh],
  );

  const signedIn = phase.name === "unlock" || phase.name === "vault";

  // The marker is only read at page load, so a reload is the only way it comes back.
  useEffect(() => {
    if (signedIn) mark(true);
  }, [signedIn]);

  useEffect(() => {
    if (phase.name === "gate") document.title = phase.word;
    else if (phase.name !== "loading") document.title = "Tuck";
  }, [phase]);

  useEffect(() => {
    if (!signedIn) return;
    const onHide = () => document.visibilityState === "hidden" && logout(LEFT_TAB);
    document.addEventListener("visibilitychange", onHide);
    return () => document.removeEventListener("visibilitychange", onHide);
  }, [signedIn, logout]);

  useEffect(() => {
    if (!signedIn) return;
    const expire = () => (phase.name === "vault" ? lock(IDLE) : logout(IDLE_UNLOCK));
    let timer = setTimeout(expire, IDLE_LOCK_MS);
    const reset = () => {
      clearTimeout(timer);
      timer = setTimeout(expire, IDLE_LOCK_MS);
    };
    const events = ["pointerdown", "keydown", "wheel", "touchstart"] as const;
    events.forEach((e) => window.addEventListener(e, reset, { passive: true }));
    return () => {
      clearTimeout(timer);
      events.forEach((e) => window.removeEventListener(e, reset));
    };
  }, [signedIn, phase.name, lock, logout]);

  switch (phase.name) {
    case "loading":
      return (
        <main className="auth">
          <Spinner label="opening tuck…" />
        </main>
      );
    case "gate":
      return <Gate word={phase.word} notice={phase.notice} onOpen={() => refresh(null)} />;
    case "auth":
      return <Auth signupOpen={!!status?.signupOpen} initialUsername={username} notice={phase.notice} onLogin={login} onSignup={signup} />;
    case "unlock":
      return <Unlock username={username} unlocker={phase.unlocker} first={phase.first} onUnlocked={unlocked} onCancel={() => logout(null)} />;
    case "vault":
      return (
        <VaultView
          vault={phase.vault}
          keyring={phase.keyring}
          loaded={phase.loaded}
          username={username}
          questions={phase.questions}
          features={status?.features ?? "both"}
          maxFileBytes={status?.maxFileBytes ?? 0}
          onRekeyed={rekeyed}
          onLock={() => lock("locked. log in and answer your questions to come back.")}
          onLogout={() => logout(null)}
        />
      );
  }
}
