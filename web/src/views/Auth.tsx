import { useState, type FormEvent } from "react";

import { QUESTION_COUNT } from "../crypto/vault";
import { Basics } from "./Basics";
import { ErrorNote, errorMessage, Field, Logo, nextFrame, questionProblem, roman, Spinner } from "./ui";

interface Props {
  signupOpen: boolean;
  initialUsername: string;
  notice: string | null;
  onLogin: (username: string, password: string) => Promise<void>;
  onSignup: (username: string, password: string, questions: string[], answers: string[]) => Promise<void>;
}

const SUGGESTIONS = [
  "what did you name your first bicycle?",
  "which song do you never skip?",
  "what was the first thing you built that worked?",
  "where would you go if you could leave tomorrow?",
  "what's the worst meal you've ever cooked?",
];

export function Brand({ line }: { line: string }) {
  return (
    <header className="brand">
      <h1 className="wordmark">
        <Logo />
        tuck
      </h1>
      <p>{line}</p>
    </header>
  );
}

export function Auth({ signupOpen, initialUsername, notice, onLogin, onSignup }: Props) {
  const [mode, setMode] = useState<"login" | "signup">(signupOpen && !initialUsername ? "signup" : "login");
  return (
    <main className="auth">
      <div className="auth-sheet">
        <Brand line="keys, hosts and files, tucked away where only you can open them." />
        {signupOpen && (
          <nav className="tabs" aria-label="log in or sign up">
            <button type="button" className={`act ${mode === "login" ? "here" : ""}`} onClick={() => setMode("login")}>
              log in
            </button>
            <button type="button" className={`act ${mode === "signup" ? "here" : ""}`} onClick={() => setMode("signup")}>
              sign up
            </button>
          </nav>
        )}
        {notice && <p className="notice">{notice}</p>}
        {mode === "login" ? <Login initialUsername={initialUsername} onLogin={onLogin} /> : <Signup onSignup={onSignup} />}
        {mode === "login" && <WhatIsTuck />}
      </div>
    </main>
  );
}

function WhatIsTuck() {
  const [open, setOpen] = useState(false);
  return (
    <div className="stack">
      <button type="button" className="act quiet" style={{ alignSelf: "flex-start" }} onClick={() => setOpen((v) => !v)} aria-expanded={open}>
        {open ? "hide what tuck is" : "what is tuck?"}
      </button>
      {open && <Basics />}
    </div>
  );
}

function Login({ initialUsername, onLogin }: { initialUsername: string; onLogin: Props["onLogin"] }) {
  const [username, setUsername] = useState(initialUsername);
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    await nextFrame();
    try {
      await onLogin(username, password);
    } catch (err) {
      setError(errorMessage(err));
      setBusy(false);
    }
  };

  return (
    <form className="stack" onSubmit={submit}>
      <div className="row">
        <Field label="username">
          <input className="line" value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" autoFocus={!initialUsername} required />
        </Field>
        <Field label="password">
          <input
            className="line"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            autoFocus={!!initialUsername}
            required
          />
        </Field>
      </div>
      <ErrorNote error={error} />
      <div className="actions">
        <button className="act go" disabled={busy}>
          {busy ? <Spinner label="deriving your key…" /> : "continue"}
        </button>
      </div>
    </form>
  );
}

function Signup({ onSignup }: { onSignup: Props["onSignup"] }) {
  const [stage, setStage] = useState<"basics" | "account" | "questions">("basics");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [questions, setQuestions] = useState<string[]>(Array(QUESTION_COUNT).fill(""));
  const [answers, setAnswers] = useState<string[]>(Array(QUESTION_COUNT).fill(""));
  const [repeats, setRepeats] = useState<string[]>(Array(QUESTION_COUNT).fill(""));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const set = (list: string[], setter: (v: string[]) => void, i: number, v: string) =>
    setter(list.map((x, j) => (j === i ? v : x)));

  const toQuestions = (e: FormEvent) => {
    e.preventDefault();
    if (password.length < 12) return setError("use at least 12 characters. a few unrelated words works well.");
    if (password !== confirm) return setError("the two passwords don't match.");
    setError(null);
    setStage("questions");
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const problem = questionProblem(questions, answers, repeats);
    if (problem) return setError(problem);
    setBusy(true);
    setError(null);
    await nextFrame();
    try {
      await onSignup(username, password, questions, answers);
    } catch (err) {
      setError(errorMessage(err));
      setBusy(false);
    }
  };

  if (stage === "basics") {
    return (
      <div className="stack">
        <span className="stamp">before you start</span>
        <Basics />
        <div className="actions">
          <button type="button" className="act go" onClick={() => setStage("account")} autoFocus>
            understood, set up my vault
          </button>
        </div>
      </div>
    );
  }

  if (stage === "account") {
    return (
      <form className="stack" onSubmit={toQuestions}>
        <Field label="username" hint="letters, digits, dot, dash or underscore">
          <input className="line" value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" autoFocus required />
        </Field>
        <div className="row">
          <Field label="password" hint="at least 12 characters">
            <input className="line" type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" required />
          </Field>
          <Field label="password again">
            <input className="line" type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="new-password" required />
          </Field>
        </div>
        <ErrorNote error={error} />
        <div className="actions">
          <button className="act go">next: your three questions</button>
        </div>
      </form>
    );
  }

  return (
    <form className="stack" onSubmit={submit}>
      <datalist id="question-ideas">
        {SUGGESTIONS.map((s) => (
          <option key={s} value={s} />
        ))}
      </datalist>
      {questions.map((q, i) => (
        <fieldset key={i} className="question-set">
          <div className="row">
            <Field label={`question ${roman(i + 1)}`} wide>
              <input
                className="line"
                list="question-ideas"
                placeholder="write a question only you can answer"
                value={q}
                onChange={(e) => set(questions, setQuestions, i, e.target.value)}
                maxLength={300}
                autoFocus={i === 0}
                required
              />
            </Field>
            <Field label="answer" hint={i === 0 ? "capitals and extra spaces don't matter" : undefined}>
              <input className="line" type="password" value={answers[i]} onChange={(e) => set(answers, setAnswers, i, e.target.value)} autoComplete="off" required />
            </Field>
            <Field label="answer again">
              <input className="line" type="password" value={repeats[i]} onChange={(e) => set(repeats, setRepeats, i, e.target.value)} autoComplete="off" required />
            </Field>
          </div>
        </fieldset>
      ))}
      <ErrorNote error={error} />
      <div className="actions">
        <button type="button" className="act quiet" onClick={() => setStage("account")} disabled={busy}>
          back
        </button>
        <button className="act go" disabled={busy}>
          {busy ? <Spinner label="deriving keys, a few seconds…" /> : "create my vault"}
        </button>
      </div>
    </form>
  );
}
