import { useState, type FormEvent, type MouseEvent } from "react";

import { api } from "../api";
import { checkKeyring, enroll, type Keyring, keyringOf, QUESTION_COUNT } from "../crypto/vault";
import { Board, ErrorNote, errorMessage, Field, nextFrame, questionProblem, roman, Spinner } from "./ui";

interface Props {
  keyring: Keyring;
  questions: string[];
  username: string;
  onRekeyed: (keyring: Keyring, questions: string[]) => void;
}

const blank = () => Array<string>(QUESTION_COUNT).fill("");

export function Access({ keyring, questions: current, username, onRekeyed }: Props) {
  const [password, setPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [newPasswordAgain, setNewPasswordAgain] = useState("");
  const [questions, setQuestions] = useState(current);
  const [answers, setAnswers] = useState(blank);
  const [newAnswers, setNewAnswers] = useState(blank);
  const [repeats, setRepeats] = useState(blank);
  const [open, setOpen] = useState<Record<string, boolean>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  const setAt = (setter: (f: (list: string[]) => string[]) => void, i: number, v: string) =>
    setter((list) => list.map((x, j) => (j === i ? v : x)));

  // The summary drives the state; letting the browser toggle as well fires a second event React can no longer read.
  const fold = (id: string) => (e: MouseEvent) => {
    e.preventDefault();
    setOpen((o) => ({ ...o, [id]: !o[id] }));
  };

  // A problem is useless behind a closed card.
  const complain = (message: string) => {
    setOpen({ password: true, ...Object.fromEntries(current.map((_, i) => [`q${i}`, true])) });
    setError(message);
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setDone(false);
    if (!password) return complain("type your current password first.");
    if (newPassword && newPassword.length < 12) return complain("a new password needs at least 12 characters.");
    if (newPassword !== newPasswordAgain) return complain("the two new passwords don't match.");
    const finals = answers.map((a, i) => newAnswers[i] || a);
    const finalRepeats = answers.map((a, i) => (newAnswers[i] ? repeats[i] : a));
    const problem = questionProblem(questions, finals, finalRepeats);
    if (problem) return complain(problem);
    const unchanged = !newPassword && newAnswers.every((a) => !a) && questions.every((q, i) => q.trim() === current[i]);
    if (unchanged) return complain("nothing has changed yet.");

    setBusy(true);
    setError(null);
    await nextFrame();
    try {
      const check = await checkKeyring(keyring, password, answers);
      if (check.wrong === 0) throw new Error("your current password is wrong.");
      if (check.wrong !== null || !check.vaultKey) throw new Error(`your current answer to question ${roman(check.wrong ?? 3)} is wrong.`);
      const enrollment = await enroll(newPassword || password, questions, finals, check.vaultKey);
      check.vaultKey?.fill(0);
      await api.rekey(check.authKey, enrollment);
      onRekeyed(keyringOf(enrollment), questions.map((q) => q.trim()));
      setDone(true);
      setPassword("");
      setNewPassword("");
      setNewPasswordAgain("");
      setAnswers(blank());
      setNewAnswers(blank());
      setRepeats(blank());
      setOpen({});
    } catch (err) {
      complain(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Board
      title="access"
      onSubmit={submit}
      action={
        <button className="act go" disabled={busy}>
          {busy ? <Spinner label="checking, then re-deriving keys…" /> : "make changes"}
        </button>
      }
      intro={
        <>
          <p className="stamp">signed in as {username}</p>
          <p className="lede muted">
            change your password, your questions, your answers, or any mix of them. your current password and answers are checked first, and new answers are
            typed twice, so a typo can't lock you out. your other devices get signed out.
          </p>
          <ErrorNote error={error} />
          {done && <p className="notice">saved. use the new password and answers from now on.</p>}
        </>
      }
    >
      <div className="register">
        <details className="question-set fold" open={!!open.password}>
          <summary onClick={fold("password")}>
            <span className="stamp">your password</span>
            <span className="fold-note">{newPassword ? "a new one is typed" : "unchanged"}</span>
          </summary>
          <div className="stack">
            <div className="row">
              <Field label="current password">
                <input className="line" type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" />
              </Field>
              <Field label="new password">
                <input
                  className="line"
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  placeholder="leave empty to keep it"
                  autoComplete="new-password"
                />
              </Field>
              <Field label="new password again">
                <input
                  className="line"
                  type="password"
                  value={newPasswordAgain}
                  onChange={(e) => setNewPasswordAgain(e.target.value)}
                  autoComplete="new-password"
                  disabled={!newPassword}
                />
              </Field>
            </div>
          </div>
        </details>
        {questions.map((q, i) => (
          <details key={i} className="question-set fold" open={!!open[`q${i}`]}>
            <summary onClick={fold(`q${i}`)}>
              <span className="stamp">question {roman(i + 1)}</span>
              <span className="fold-note">{q}</span>
            </summary>
            <div className="stack">
              <Field label="the question" wide>
                <input className="line" value={q} onChange={(e) => setAt(setQuestions, i, e.target.value)} maxLength={300} />
              </Field>
              <div className="row">
                <Field label="current answer">
                  <input className="line" type="password" value={answers[i]} onChange={(e) => setAt(setAnswers, i, e.target.value)} autoComplete="off" />
                </Field>
                <Field label="new answer">
                  <input
                    className="line"
                    type="password"
                    value={newAnswers[i]}
                    onChange={(e) => setAt(setNewAnswers, i, e.target.value)}
                    placeholder="leave empty to keep it"
                    autoComplete="off"
                  />
                </Field>
                <Field label="new answer again">
                  <input
                    className="line"
                    type="password"
                    value={repeats[i]}
                    onChange={(e) => setAt(setRepeats, i, e.target.value)}
                    autoComplete="off"
                    disabled={!newAnswers[i]}
                  />
                </Field>
              </div>
            </div>
          </details>
        ))}
      </div>
    </Board>
  );
}
