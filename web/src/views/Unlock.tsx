import { useEffect, useRef, useState, type FormEvent } from "react";

import { api, ApiError, type StepResponse } from "../api";
import { type Keyring, QUESTION_COUNT, type Unlocker, type Vault } from "../crypto/vault";
import { Brand } from "./Auth";
import { Sky } from "./Sky";
import { ErrorNote, errorMessage, nextFrame, roman, Spinner } from "./ui";

interface Props {
  username: string;
  unlocker: Unlocker;
  first: StepResponse;
  onUnlocked: (vault: Vault, keyring: Keyring, questions: string[]) => Promise<void>;
  onCancel: () => void;
}

const WORDS = ["one", "two", "three", "four", "five"];

export function Unlock({ username, unlocker, first, onUnlocked, onCancel }: Props) {
  const [current, setCurrent] = useState(first);
  const [question, setQuestion] = useState<string | null>(null);
  const [answer, setAnswer] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lockedFor, setLockedFor] = useState(0);
  const [shake, setShake] = useState(0);
  const [loadFailed, setLoadFailed] = useState(false);
  const [frozen, setFrozen] = useState(false);
  const asked = useRef<string[]>([]);
  const input = useRef<HTMLInputElement>(null);

  useEffect(() => {
    let live = true;
    unlocker
      .readQuestion(current.question)
      .then((q) => live && setQuestion(q))
      .catch(() => live && setError("this question couldn't be decrypted. log in again."));
    return () => {
      live = false;
    };
  }, [current, unlocker]);

  useEffect(() => {
    if (lockedFor <= 0) return;
    const t = setTimeout(() => setLockedFor((s) => s - 1), 1000);
    return () => clearTimeout(t);
  }, [lockedFor]);

  useEffect(() => input.current?.focus(), [question, busy]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!question || lockedFor > 0 || loadFailed || frozen) return;
    setBusy(true);
    setError(null);
    await nextFrame();
    try {
      const { proof, next } = await unlocker.attempt(answer, current.question);
      const res = await api.answer(unlocker.nextStep, proof);
      unlocker.accept(next, current.question);
      asked.current.push(question);
      setAnswer("");
      if ("wrappedKey" in res) {
        const { vault, keyring } = await unlocker.finish(res.wrappedKey, res.wrappedNonce);
        await onUnlocked(vault, keyring, asked.current);
        return;
      }
      setQuestion(null);
      setCurrent(res);
    } catch (err) {
      if (unlocker.nextStep > QUESTION_COUNT) {
        setLoadFailed(true);
        setError(`your answers were right, but the vault couldn't be loaded: ${errorMessage(err)} log out and try again.`);
        return;
      }
      setShake((n) => n + 1);
      setAnswer("");
      if (err instanceof ApiError && err.status === 423 && err.body.frozen) {
        setFrozen(true);
        setError(errorMessage(err));
      } else if (err instanceof ApiError && err.status === 429) {
        setLockedFor(Number(err.body.retryAfter ?? 60));
      } else if (err instanceof ApiError && err.status === 403) {
        const left = Number(err.body.attemptsBeforeLock);
        const more = left === 1 ? "one more wrong answer" : `${WORDS[left - 1] ?? left} more wrong answers`;
        const then = err.body.freezesNext
          ? "will freeze unlocking until whoever runs tuck lifts it in the database"
          : "will pause unlocking, for longer each time";
        setError(`that's not the answer. ${more} ${then}.`);
      } else {
        setError(errorMessage(err));
      }
    } finally {
      setBusy(false);
    }
  };

  const minutes = Math.ceil(lockedFor / 60);
  const wait = minutes >= 120 ? `${Math.ceil(minutes / 60)} hours` : `${minutes} minute${minutes === 1 ? "" : "s"}`;
  return (
    <main className="auth">
      <Sky />
      <div className="auth-sheet">
        <Brand line={`unlocking ${username}'s vault`} />
        <div className="stack" style={{ gap: ".9rem" }}>
          <ol className="temper" aria-label={`question ${current.step} of ${QUESTION_COUNT}`}>
            {Array.from({ length: QUESTION_COUNT }, (_, i) => (
              <li key={i} className={i + 1 > current.step ? "ahead" : ""} />
            ))}
          </ol>
          <span className="stamp">
            question {roman(current.step)} of {roman(QUESTION_COUNT)}
          </span>
        </div>
        <form className="stack" onSubmit={submit}>
          <p key={`${current.step}-${shake}`} className={`question ${shake ? "shake" : ""}`}>
            {question ?? <Spinner label="decrypting…" />}
          </p>
          <label className="field">
            <span className="stamp">your answer</span>
            <input
              ref={input}
              className="line"
              type="password"
              value={answer}
              onChange={(e) => setAnswer(e.target.value)}
              disabled={busy || !question || lockedFor > 0 || loadFailed || frozen}
              autoComplete="off"
              required
            />
          </label>
          <ErrorNote
            error={lockedFor > 0 ? `too many wrong answers. try again in ${wait}.` : error}
          />
          <div className="actions">
            <button type="button" className="act quiet" onClick={onCancel} disabled={busy}>
              log out
            </button>
            <button className="act go" disabled={busy || !question || lockedFor > 0 || loadFailed || frozen}>
              {busy ? <Spinner label="checking…" /> : current.step === QUESTION_COUNT ? "unlock" : "next"}
            </button>
          </div>
        </form>
      </div>
    </main>
  );
}
