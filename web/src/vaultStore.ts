import { useCallback, useMemo, useRef, useState } from "react";

import { api, ApiError, type StoredItem } from "./api";
import { fromBase64, toBase64 } from "./crypto/encoding";
import type { Vault } from "./crypto/vault";
import type { CredentialItem, FileItem, HostItem, Item, ItemKind } from "./types";

type Payload = CredentialItem | HostItem | FileItem;

export const MANIFEST_ID = "00000000-0000-4000-8000-000000000000";
const SEEN_KEY = "tuck:manifest-counter";

interface Envelope {
  v: number;
  data: Payload;
  blob?: string;
}

export interface Manifest {
  counter: number;
  items: Record<string, { kind: ItemKind; v: number }>;
}

export interface ManifestState {
  manifest: Manifest;
  hash: string;
}

export interface Versioned extends Item {
  v: number;
  blob?: string;
  hash: string;
}

export interface Loaded {
  items: Versioned[];
  state: ManifestState;
  problems: string[];
}

export interface VaultData {
  credentials: { id: string; data: CredentialItem }[];
  hosts: { id: string; data: HostItem }[];
  files: { id: string; data: FileItem }[];
  problems: string[];
  save: (kind: ItemKind, data: Payload, id?: string) => Promise<string>;
  remove: (id: string) => Promise<void>;
  saveFile: (data: FileItem, bytes: Uint8Array) => Promise<void>;
  readFile: (id: string) => Promise<Uint8Array>;
}

const sha256 = async (bytes: Uint8Array) => toBase64(new Uint8Array(await crypto.subtle.digest("SHA-256", bytes as Uint8Array<ArrayBuffer>)));

const plural = (n: number, one: string, many: string) => (n === 1 ? one : many.replace("#", String(n)));

function lastSeen(): number {
  try {
    return Number(localStorage.getItem(SEEN_KEY) ?? 0) || 0;
  } catch {
    return 0;
  }
}

function forget() {
  try {
    localStorage.removeItem(SEEN_KEY);
  } catch {
  }
}

function remember(counter: number) {
  try {
    if (counter > lastSeen()) localStorage.setItem(SEEN_KEY, String(counter));
  } catch {
  }
}

async function readManifest(vault: Vault, stored: StoredItem[]): Promise<ManifestState | null> {
  const m = stored.find((s) => s.id === MANIFEST_ID && s.kind === "manifest");
  if (!m) return null;
  const manifest = await vault.decryptItem<Manifest>(m.id, m.kind, m.nonce, m.ciphertext);
  return { manifest, hash: await sha256(fromBase64(m.ciphertext)) };
}

export async function loadItems(vault: Vault): Promise<Loaded> {
  const { items: stored } = await api.items();
  const others = stored.filter((s) => s.kind !== "manifest");
  const state = await readManifest(vault, stored);
  if (!state) {
    if (others.length > 0) {
      throw new Error("the vault's index is missing, so tuck can't tell whether anything in it is current. nothing was opened.");
    }
    const problems = lastSeen() > 0 ? ["the server says this vault is empty, but this browser has seen things in it before."] : [];
    forget();
    return { items: [], state: { manifest: { counter: 0, items: {} }, hash: "" }, problems };
  }

  const { manifest } = state;
  const problems: string[] = [];
  if (manifest.counter < lastSeen()) {
    problems.push("the server sent an older copy of the whole vault than this browser has seen before. recent changes may be missing.");
  }
  remember(manifest.counter);

  let stale = 0;
  const items: Versioned[] = [];
  for (const s of others) {
    const want = manifest.items[s.id];
    if (!want) continue;
    if (want.kind !== s.kind) {
      stale++;
      continue;
    }
    const env = await vault.decryptItem<Envelope>(s.id, s.kind, s.nonce, s.ciphertext);
    if (env.v < want.v) {
      stale++;
      continue;
    }
    const hash = await sha256(fromBase64(s.ciphertext));
    items.push({ id: s.id, kind: s.kind as ItemKind, updatedAt: s.updatedAt, data: env.data, v: env.v, blob: env.blob, hash });
  }
  const missing = Object.keys(manifest.items).filter((id) => !others.some((s) => s.id === id)).length;
  if (stale) problems.push(plural(stale, "one item came back older or altered, so tuck is hiding it.", "# items came back older or altered, so tuck is hiding them."));
  if (missing) problems.push(plural(missing, "one item is missing from the server.", "# items are missing from the server."));
  return { items, state, problems };
}

export function useVaultData(vault: Vault, loaded: Loaded): VaultData {
  const [items, setItems] = useState<Versioned[]>(loaded.items);
  const state = useRef(loaded.state);

  const commit = useCallback(
    async (change: (m: Manifest) => void) => {
      for (let attempt = 0; ; attempt++) {
        const current = state.current;
        const next: Manifest = { counter: current.manifest.counter + 1, items: { ...current.manifest.items } };
        change(next);
        const sealed = await vault.encryptItem(MANIFEST_ID, "manifest", next);
        try {
          await api.putItem(MANIFEST_ID, "manifest", sealed.nonce, sealed.ciphertext, current.hash);
          state.current = { manifest: next, hash: await sha256(fromBase64(sealed.ciphertext)) };
          remember(next.counter);
          return;
        } catch (err) {
          if (!(err instanceof ApiError) || err.status !== 412 || attempt >= 3) throw err;
          const fresh = await readManifest(vault, (await api.items()).items);
          if (!fresh) throw err;
          state.current = fresh;
        }
      }
    },
    [vault],
  );

  const write = useCallback(
    async (kind: ItemKind, data: Payload, id: string, blob?: Uint8Array) => {
      if (state.current.hash === "") await commit(() => {});
      const previous = items.find((i) => i.id === id);
      const v = Math.max(state.current.manifest.items[id]?.v ?? 0, previous?.v ?? 0) + 1;
      const blobHash = blob ? await sha256(blob) : previous?.blob;
      const sealed = await vault.encryptItem(id, kind, { v, data, blob: blobHash } satisfies Envelope);
      await api.putItem(id, kind, sealed.nonce, sealed.ciphertext, previous?.hash ?? "");
      if (blob) await api.putFile(id, blob);
      await commit((m) => {
        m.items[id] = { kind, v };
      });
      const hash = await sha256(fromBase64(sealed.ciphertext));
      const next: Versioned = { id, kind, data, v, blob: blobHash, hash, updatedAt: new Date().toISOString() };
      setItems((prev) => (prev.some((i) => i.id === id) ? prev.map((i) => (i.id === id ? next : i)) : [...prev, next]));
    },
    [vault, items, commit],
  );

  const save = useCallback(
    async (kind: ItemKind, data: Payload, id: string = crypto.randomUUID()) => {
      await write(kind, data, id);
      return id;
    },
    [write],
  );

  const remove = useCallback(
    async (id: string) => {
      await commit((m) => {
        delete m.items[id];
      });
      setItems((prev) => prev.filter((i) => i.id !== id));
      await api.deleteItem(id).catch(() => {});
    },
    [commit],
  );

  const saveFile = useCallback(
    async (data: FileItem, bytes: Uint8Array) => {
      const id = crypto.randomUUID();
      try {
        await write("file", data, id, await vault.encryptFile(id, bytes));
      } catch (err) {
        await api.deleteItem(id).catch(() => {});
        throw err;
      }
    },
    [vault, write],
  );

  const readFile = useCallback(
    async (id: string) => {
      const blob = await api.getFile(id);
      const want = items.find((i) => i.id === id)?.blob;
      if (!want || (await sha256(blob)) !== want) throw new Error("this file's contents aren't the ones that were saved, so tuck won't open them.");
      return vault.decryptFile(id, blob);
    },
    [vault, items],
  );

  const lists = useMemo(() => {
    const of = <T,>(kind: ItemKind) => items.filter((i) => i.kind === kind).map((i) => ({ id: i.id, data: i.data as T }));
    return { credentials: of<CredentialItem>("credential"), hosts: of<HostItem>("host"), files: of<FileItem>("file") };
  }, [items]);
  return { ...lists, problems: loaded.problems, save, remove, saveFile, readFile };
}
