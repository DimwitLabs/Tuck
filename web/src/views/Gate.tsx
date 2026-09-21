import { useEffect, useRef, useState, type SyntheticEvent } from "react";

import { api } from "../api";

interface Props {
  word: string;
  notice: string | null;
  onOpen: () => void;
}

const KEEP = 256;

export function Gate({ word, notice, onOpen }: Props) {
  const [typed, setTyped] = useState("");
  const [showNotice, setShowNotice] = useState(!!notice);
  const opened = useRef(false);

  useEffect(() => {
    if (!notice) return;
    const t = setTimeout(() => setShowNotice(false), 6000);
    return () => clearTimeout(t);
  }, [notice]);

  const update = (next: string) => {
    const text = next.slice(-KEEP);
    setTyped(text);
    if (!text) return;
    api
      .gate(text)
      .then(({ open }) => {
        if (open && !opened.current) {
          opened.current = true;
          onOpen();
        }
      })
      .catch(() => {});
  };

  const refuse = (e: SyntheticEvent) => e.preventDefault();

  return (
    <main className="gate" onDragOver={refuse} onDrop={refuse}>
      <p className="gate-word" aria-hidden>
        {word}
      </p>
      <input
        className="gate-input"
        type="password"
        aria-label="tuck"
        value={typed}
        onChange={(e) => update(e.target.value)}
        onPaste={refuse}
        onDrop={refuse}
        autoComplete="off"
        autoFocus
      />
      {notice && (
        <p className={`gate-notice ${showNotice ? "" : "gone"}`} role="status">
          {notice}
        </p>
      )}
    </main>
  );
}
