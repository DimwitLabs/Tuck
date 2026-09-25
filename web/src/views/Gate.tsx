import { useCallback, useEffect, useRef, useState, type SyntheticEvent } from "react";

import { api } from "../api";
import { credentialJSON, deviceEnrolled, requestOptions } from "../passkey";

interface Props {
  word: string;
  notice: string | null;
  onOpen: () => void;
}

const KEEP = 256;

const touch = typeof matchMedia !== "undefined" && matchMedia("(pointer: coarse)").matches;

const HOLD = 600;

export function Gate({ word, notice, onOpen }: Props) {
  const field = useRef<HTMLInputElement>(null);
  const [typed, setTyped] = useState("");
  const [showNotice, setShowNotice] = useState(!!notice);
  const opened = useRef(false);
  const hold = useRef<ReturnType<typeof setTimeout>>(undefined);

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

  // Nothing on the door says a passkey exists. A browser that enrolled one is asked on arrival; anyone else holds the word.
  const askForPasskey = useCallback(async () => {
    if (typeof PublicKeyCredential === "undefined" || !window.isSecureContext || opened.current) return;
    try {
      const options = requestOptions((await api.gatePasskeyStart()) as never);
      const credential = (await navigator.credentials.get({ publicKey: options })) as PublicKeyCredential | null;
      if (!credential) return;
      const { open } = await api.gatePasskeyFinish(credentialJSON(credential));
      if (open && !opened.current) {
        opened.current = true;
        onOpen();
      }
    } catch {
      // a door that answers is a door that admits it is there
    } finally {
      if (!opened.current && !touch) field.current?.focus();
    }
  }, [onOpen]);

  useEffect(() => {
    if (deviceEnrolled()) void askForPasskey();
  }, [askForPasskey]);

  const holding = (down: boolean) => {
    if (!down) {
      clearTimeout(hold.current);
      return;
    }
    hold.current = setTimeout(() => void askForPasskey(), HOLD);
  };

  return (
    <main
      className="gate"
      onDragOver={refuse}
      onDrop={refuse}
      onClick={() => field.current?.focus()}
      onPointerDown={() => holding(true)}
      onPointerUp={() => holding(false)}
      onPointerCancel={() => holding(false)}
      onPointerLeave={() => holding(false)}
      onContextMenu={refuse}
    >
      <p className="gate-word" aria-hidden>
        {word}
      </p>
      <input
        ref={field}
        className="gate-input"
        type="text"
        aria-label="tuck"
        value={typed}
        onChange={(e) => update(e.target.value)}
        onPaste={refuse}
        onDrop={refuse}
        autoComplete="off"
        autoCorrect="off"
        autoCapitalize="off"
        spellCheck={false}
        data-1p-ignore
        data-lpignore="true"
        data-bwignore
        data-protonpass-ignore
        autoFocus={!touch}
      />
      {notice && (
        <p className={`gate-notice ${showNotice ? "" : "gone"}`} role="status">
          {notice}
        </p>
      )}
    </main>
  );
}
