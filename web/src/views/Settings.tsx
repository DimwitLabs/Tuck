import { useState, type FormEvent } from "react";

import { api } from "../api";
import { checkKeyring, enroll, type Keyring, keyringOf, QUESTION_COUNT } from "../crypto/vault";
import { ErrorNote, errorMessage, Field, nextFrame, questionProblem, roman, Spinner } from "./ui";

interface Props {
  keyring: Keyring;
  questions: string[];
  onRekeyed: (keyring: Keyring, questions: string[]) => void;
}

export function Settings({ keyring, questions, onRekeyed }: Props) {
  return (
    <section className="panel">
      <Rekey keyring={keyring} current={questions} onRekeyed={onRekeyed} />
    </section>
  );
}

const blank = () => Array<string>(QUESTION_COUNT).fill("");

function Rekey({ keyring, current, onRekeyed }: Omit<Props, "questions"> & { current: string[] }) {
  const [password, setPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [newPasswordAgain, setNewPasswordAgain] = useState("");
  const [questions, setQuestions] = useState(current);
  const [answers, setAnswers] = useState(blank);
  const [newAnswers, setNewAnswers] = useState(blank);
  const [repeats, setRepeats] = useState(blank);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  const setAt = (setter: (f: (list: string[]) => string[]) => void, i: number, v: string) =>
    setter((list) => list.map((x, j) => (j === i ? v : x)));

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setDone(false);
    if (newPassword && newPassword.length < 12) return setError("a new password needs at least 12 characters.");
    if (newPassword !== newPasswordAgain) return setError("the two new passwords don't match.");
    const finals = answers.map((a, i) => newAnswers[i] || a);
    const finalRepeats = answers.map((a, i) => (newAnswers[i] ? repeats[i] : a));
    const problem = questionProblem(questions, finals, finalRepeats);
    if (problem) return setError(problem);
    const unchanged = !newPassword && newAnswers.every((a) => !a) && questions.every((q, i) => q.trim() === current[i]);
    if (unchanged) return setError("nothing has changed yet.");

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
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form className="stack" onSubmit={submit}>
      <div className="section-head">
        <h2>password and questions</h2>
      </div>
      <p className="lede muted">
        change your password, your questions, your answers, or any mix of them. your current password and answers are
        checked first, and new answers are typed twice, so a typo can't lock you out. your other devices get signed out.
      </p>
      <div className="row">
        <Field label="current password">
          <input className="line" type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" required />
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
      {questions.map((q, i) => (
        <fieldset key={i} className="question-set">
          <div className="row">
            <Field label={`question ${roman(i + 1)}`} wide>
              <input className="line" value={q} onChange={(e) => setAt(setQuestions, i, e.target.value)} maxLength={300} required />
            </Field>
            <Field label="current answer">
              <input className="line" type="password" value={answers[i]} onChange={(e) => setAt(setAnswers, i, e.target.value)} autoComplete="off" required />
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
        </fieldset>
      ))}
      <ErrorNote error={error} />
      {done && <p className="notice">saved. use the new password and answers from now on.</p>}
      <div className="actions end">
        <button className="act go" disabled={busy}>
          {busy ? <Spinner label="checking, then re-deriving keys…" /> : "save"}
        </button>
      </div>
    </form>
  );
}
