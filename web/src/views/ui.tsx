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

export function Logo() {
  return (
    <svg className="logo" viewBox="0 0 64 64" aria-hidden focusable="false">
      <g transform="matrix(0.3047 0 0 0.3047 -8.07 -6.19)">
        <path className="logo-pillow" d="M200.311,75.035l0,16.523c0,17.732 -5.217,32.127 -22.948,32.127l-89.957,0c-17.732,0 -22.948,-11.964 -22.948,-29.695c-0,-0 -0,-16.523 -0,-16.523c0,-17.732 5.217,-34.56 22.948,-34.56l89.957,0c17.732,0 22.948,14.396 22.948,32.127Z" />
        <path className="logo-cat" d="M81.148,128.471l-3.778,-45.917l17.29,-31.649l21.359,17.801l26.347,-2.168l18.162,-21.053l22.231,28.397l3.778,45.917l-105.389,8.672Z" strokeWidth="10.28" strokeLinejoin="round" />
        <path className="logo-eyes" d="M99.648,91.389c7.129,6.051 13.716,5.509 19.76,-1.626m23.054,-1.897c7.129,6.051 13.716,5.509 19.76,-1.626" strokeWidth="7.54" strokeLinecap="round" />
        <path className="logo-sheet" d="M69.075,125.409c-0.001,-5.537 3.57,-10.443 8.839,-12.146c36.031,-11.306 72.046,-11.306 108.062,0.05c5.248,1.695 8.804,6.581 8.804,12.096c0.051,23.122 0.051,82.436 0.051,82.436c0,10.61 -4.412,15.915 -13.237,15.915l-99.281,0c-8.825,0 -13.237,-5.305 -13.237,-15.915l0,-82.436Z" />
        <path className="logo-whiskers" d="M82.421,105.92l-28.523,-0.004m30.244,9.767l-27.088,8.132m122.996,-35.11l26.801,-9.759m-25.08,19.522l28.236,-1.623" strokeWidth="4.96" strokeLinecap="round" />
        <path className="logo-fold" d="M69.075,123.365c-0.001,-4.82 2.977,-9.138 7.482,-10.851c36.937,-13.726 73.855,-13.726 110.773,0.048c4.486,1.705 7.451,6.004 7.451,10.802c0.05,8.845 0.05,22.038 0.05,22.038c-41.919,-17.758 -83.837,-17.758 -125.756,0l0,-22.038Z" />
        <ellipse className="logo-paw" cx="103.357" cy="111.772" rx="13.952" ry="9.546" />
        <ellipse className="logo-paw" cx="158.432" cy="109.569" rx="13.952" ry="9.546" />
      </g>
    </svg>
  );
}
