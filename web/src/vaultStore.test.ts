import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { StoredItem } from "./api";
import { randomBytes } from "./crypto/encoding";
import { Vault } from "./crypto/vault";
import { loadItems, MANIFEST_ID, type Manifest } from "./vaultStore";

let stored: StoredItem[] = [];
vi.mock("./api", async (real) => ({
  ...(await real<typeof import("./api")>()),
  api: { items: async () => ({ items: stored }) },
}));

const A = "7f1c2a3b-4d5e-4f60-8a9b-0c1d2e3f4a5b";
const B = "7f1c2a3b-4d5e-4f60-8a9b-0c1d2e3f4a5c";

describe("loading a versioned vault", () => {
  let vault: Vault;
  let memory: Record<string, string>;

  const row = async (id: string, kind: StoredItem["kind"], payload: unknown): Promise<StoredItem> => ({
    id,
    kind,
    ...(await vault.encryptItem(id, kind, payload)),
    hasFile: false,
    updatedAt: "2026-09-21T00:00:00Z",
  });
  const manifest = (m: Manifest) => row(MANIFEST_ID, "manifest", m);
  const host = (id: string, v: number, name: string) => row(id, "host", { v, data: { name, aliases: [], hostname: "h", tags: [] } });

  beforeEach(async () => {
    vault = await Vault.fromRaw(randomBytes(32));
    memory = {};
    vi.stubGlobal("localStorage", {
      getItem: (k: string) => memory[k] ?? null,
      setItem: (k: string, v: string) => (memory[k] = v),
      removeItem: (k: string) => delete memory[k],
    });
  });
  afterEach(() => vi.unstubAllGlobals());

  it("shows items at or past the version the manifest expects", async () => {
    stored = [await manifest({ counter: 2, items: { [A]: { kind: "host", v: 2 } } }), await host(A, 3, "newer")];
    const { items, problems } = await loadItems(vault);
    expect(items.map((i) => i.data.name)).toEqual(["newer"]);
    expect(problems).toEqual([]);
  });

  it("hides an older copy, a deleted item and a kind that doesn't match, and says what is missing", async () => {
    stored = [
      await manifest({ counter: 5, items: { [A]: { kind: "host", v: 2 }, [B]: { kind: "host", v: 1 }, "gone-id": { kind: "host", v: 1 } } }),
      await host(A, 1, "rolled back"),
      await row(B, "credential", { v: 1, data: { name: "swapped kind", tags: [] } }),
      await host("7f1c2a3b-4d5e-4f60-8a9b-0c1d2e3f4a5d", 1, "deleted"),
    ];
    const { items, problems } = await loadItems(vault);
    expect(items).toEqual([]);
    expect(problems.join(" ")).toMatch(/2 items came back older or altered/);
    expect(problems.join(" ")).toMatch(/one item is missing/);
  });

  it("notices the whole vault going back in time for this browser", async () => {
    stored = [await manifest({ counter: 9, items: {} })];
    await loadItems(vault);
    stored = [await manifest({ counter: 4, items: {} })];
    expect((await loadItems(vault)).problems.join(" ")).toMatch(/older copy of the whole vault/);
  });

  it("keeps warning about a vault that came back empty until something new is saved", async () => {
    stored = [await manifest({ counter: 9, items: {} })];
    await loadItems(vault);
    stored = [];
    expect((await loadItems(vault)).problems.join(" ")).toMatch(/seen things in it before/);
    expect((await loadItems(vault)).problems.join(" ")).toMatch(/seen things in it before/);
  });

  it("refuses to open items with no manifest to vouch for them", async () => {
    stored = [await host(A, 1, "unvouched")];
    await expect(loadItems(vault)).rejects.toThrow(/index is missing/);
  });
});
