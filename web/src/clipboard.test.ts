import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { CLEAR_AFTER_MS, copyText, forgetClipboard } from "./clipboard";

describe("clipboard", () => {
  let board = "";
  let focused = true;
  let onFocus: (() => void) | undefined;

  beforeEach(() => {
    vi.useFakeTimers();
    board = "";
    focused = true;
    vi.stubGlobal("navigator", {
      clipboard: {
        writeText: async (t: string) => {
          if (!focused) throw new Error("NotAllowedError");
          board = t;
        },
      },
    });
    vi.stubGlobal("window", { addEventListener: (_: string, f: () => void) => (onFocus = f), removeEventListener: () => {} });
  });
  afterEach(() => {
    forgetClipboard();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("clears a secret after 30 seconds", async () => {
    await copyText("hunter2", true);
    await vi.advanceTimersByTimeAsync(CLEAR_AFTER_MS - 1);
    expect(board).toBe("hunter2");
    await vi.advanceTimersByTimeAsync(1);
    expect(board).toBe("");
  });

  it("leaves ordinary text alone, and a later copy cancels the clear", async () => {
    await copyText("hunter2", true);
    await copyText("ssh box", false);
    await vi.advanceTimersByTimeAsync(CLEAR_AFTER_MS);
    expect(board).toBe("ssh box");
  });

  it("clears at once on lock, or when focus comes back", async () => {
    await copyText("hunter2", true);
    focused = false;
    forgetClipboard();
    await vi.advanceTimersByTimeAsync(0);
    expect(board).toBe("hunter2");
    focused = true;
    onFocus?.();
    await vi.advanceTimersByTimeAsync(0);
    expect(board).toBe("");
  });
});
