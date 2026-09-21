export const CLEAR_AFTER_MS = 30_000;
let pendingClear: ReturnType<typeof setTimeout> | undefined;

export async function copyText(text: string, secret: boolean) {
  await navigator.clipboard.writeText(text);
  clearTimeout(pendingClear);
  pendingClear = secret ? setTimeout(forgetClipboard, CLEAR_AFTER_MS) : undefined;
}

export function forgetClipboard() {
  if (pendingClear === undefined) return;
  clearTimeout(pendingClear);
  pendingClear = undefined;
  clear();
}

const RETRY_ON = ["focus", "pointerdown", "keydown"] as const;

function clear() {
  navigator.clipboard.writeText("").catch(() => {
    const retry = () => {
      RETRY_ON.forEach((e) => window.removeEventListener(e, retry, true));
      clear();
    };
    RETRY_ON.forEach((e) => window.addEventListener(e, retry, { once: true, capture: true }));
  });
}
