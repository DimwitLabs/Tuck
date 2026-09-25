import { useRef } from "react";
import type { Props } from "@theme/ColorModeToggle";

export default function ColorModeToggle({ className, value, onChange }: Props) {
  const sounds = useRef<Record<string, HTMLAudioElement>>({});

  const flick = () => {
    const next = value === "dark" ? "light" : "dark";
    onChange(next);
    const sound = (sounds.current[next] ??= new Audio(next === "light" ? "/sfx/lamp-on.mp3" : "/sfx/lamp-off.mp3"));
    sound.currentTime = 0;
    void sound.play().catch(() => {});
  };

  return (
    <button type="button" className={`flick ${className ?? ""}`} onClick={flick} aria-label="switch theme" title="switch theme">
      <span className="lever" />
    </button>
  );
}
