import { useEffect, useState, type ReactNode } from "react";

import { copyText } from "../clipboard";
import { normalizeAnswer } from "../crypto/vault";

export function Field({ label, hint, wide, children }: { label: string; hint?: ReactNode; wide?: boolean; children: ReactNode }) {
  return (
    <label className={wide ? "field wide" : "field"}>
      <span className="stamp">{label}</span>
      {children}
      {hint && <span className="hint">{hint}</span>}
    </label>
  );
}

export function ErrorNote({ error }: { error: string | null }) {
  return error ? (
    <p className="error" role="alert">
      {error}
    </p>
  ) : null;
}

export function Spinner({ label }: { label: string }) {
  return (
    <span className="spinner" role="status">
      <span className="spinner-dot" aria-hidden />
      {label}
    </span>
  );
}

export function CopyButton({ text, label = "copy", secret = false }: { text: string; label?: string; secret?: boolean }) {
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const t = setTimeout(() => setCopied(false), secret ? 2400 : 1400);
    return () => clearTimeout(t);
  }, [copied, secret]);
  return (
    <button
      type="button"
      className="act"
      onClick={async () => {
        await copyText(text, secret);
        setCopied(true);
      }}
    >
      {copied ? (secret ? "copied. clear it once pasted" : "copied") : label}
    </button>
  );
}

export function Sheet({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      window.removeEventListener("keydown", onKey);
      document.body.style.overflow = overflow;
    };
  }, [onClose]);
  return (
    <div className="overlay">
      <div className="editor" role="dialog" aria-modal aria-label={title}>
        <header className="section-head">
          <h2>{title}</h2>
          <button type="button" className="act quiet" onClick={onClose}>
            close
          </button>
        </header>
        {children}
      </div>
    </div>
  );
}

export const splitList = (s: string) =>
  s
    .split(/[\s,]+/)
    .map((t) => t.trim())
    .filter(Boolean);

export function Stamps({ items }: { items: (string | false | undefined)[] }) {
  const shown = items.filter(Boolean) as string[];
  return shown.length ? (
    <p className="stamps">
      {shown.map((t) => (
        <span key={t} className="stamp">
          {t}
        </span>
      ))}
    </p>
  ) : null;
}

export const errorMessage = (err: unknown) => (err instanceof Error ? err.message : String(err)).toLowerCase();

export const nextFrame = () => new Promise((r) => setTimeout(r, 30));

export const roman = (n: number) => ["i", "ii", "iii"][n - 1] ?? String(n);

export function Search({ value, onChange, what }: { value: string; onChange: (v: string) => void; what: string }) {
  return (
    <input
      className="line search"
      type="search"
      aria-label={`search ${what}`}
      placeholder={`search ${what} by name, alias or tag`}
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  );
}

export function downloadBytes(bytes: Uint8Array | string, name: string, type = "application/octet-stream") {
  const url = URL.createObjectURL(new Blob([bytes as BlobPart], { type }));
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export const megabytes = (n: number) => `${Math.round(n / (1 << 20))} mb`;

export function questionProblem(questions: string[], answers: string[], repeats: string[]): string | null {
  if (new Set(questions.map(normalizeAnswer)).size !== questions.length) return "use three different questions.";
  const mismatch = answers.findIndex((a, i) => normalizeAnswer(a) !== normalizeAnswer(repeats[i]));
  if (mismatch >= 0) return `answer ${roman(mismatch + 1)} doesn't match when typed again.`;
  if (answers.some((a) => normalizeAnswer(a).length < 2)) return "each answer needs at least two characters.";
  return null;
}

const TILT = "translate(0 -1) rotate(-10 32 28) translate(32 28) scale(0.9) translate(-32 -28)";

export function Logo() {
  return (
    <svg className="logo" viewBox="0 0 64 64" aria-hidden focusable="false">
      <rect className="logo-pillow" x="11" y="6" width="42" height="22" rx="7" />
      <g transform={TILT}>
        <path className="logo-cat" d="M16 38V25L22 16.5L28 22H36L42 16.5L48 25V38Z" strokeWidth="3" strokeLinejoin="round" />
        <path className="logo-eyes" d="M22.5 28Q25.5 30.8 28.5 28M35.5 28Q38.5 30.8 41.5 28" strokeWidth="2.2" strokeLinecap="round" />
      </g>
      <path className="logo-sheet" d="M13 35Q32 29.2 51 35V58Q51 62 47 62H17Q13 62 13 58Z" />
      <path className="logo-fold" d="M13 35Q32 29.2 51 35V41.5Q32 35.7 13 41.5Z" />
      <ellipse className="logo-paw" cx="24.5" cy="34" rx="3.8" ry="2.6" />
      <ellipse className="logo-paw" cx="39.5" cy="33.4" rx="3.8" ry="2.6" />
      <g transform={TILT}>
        <path className="logo-whiskers" d="M17 31L8.5 29.5M17 34L8.5 35M47 31L55.5 29.5M47 34L55.5 35" strokeWidth="1.5" strokeLinecap="round" />
      </g>
    </svg>
  );
}
