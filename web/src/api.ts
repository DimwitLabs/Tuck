import type { Enrollment, KdfParams, StepPublic } from "./crypto/vault";
import type { ItemKind } from "./types";

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
    readonly body: Record<string, unknown> = {},
  ) {
    super(message);
  }
}

async function request<T>(method: string, path: string, body?: unknown, raw?: Uint8Array, keepalive = false): Promise<T> {
  const headers: Record<string, string> = { "X-Tuck-Request": "1" };
  let payload: BodyInit | undefined;
  if (raw) {
    headers["Content-Type"] = "application/octet-stream";
    payload = raw as Uint8Array<ArrayBuffer>;
  } else if (body !== undefined) {
    headers["Content-Type"] = "application/json";
    payload = JSON.stringify(body);
  }
  const res = await fetch(path, { method, headers, body: payload, credentials: "same-origin", keepalive });
  if (res.status === 204) return undefined as T;
  if (res.headers.get("Content-Type")?.includes("application/octet-stream")) {
    return new Uint8Array(await res.arrayBuffer()) as T;
  }
  const json = (await res.json().catch(() => ({}))) as Record<string, unknown>;
  if (!res.ok) throw new ApiError(res.status, String(json.error ?? res.statusText), json);
  return json as T;
}

export type Status =
  | { gate: true; word: string }
  | {
      gate: false;
      signupOpen: boolean;
      features: Features;
      maxFileBytes: number;
      session: { username: string; unlocked: boolean } | null;
    };

export type Features = "both" | "ssh" | "files";

export type OpenStatus = Extract<Status, { gate: false }>;

export interface StepResponse {
  step: number;
  question: StepPublic;
}

export interface FinalResponse {
  wrappedKey: string;
  wrappedNonce: string;
}

export type StoredKind = ItemKind | "manifest";

export interface StoredItem {
  id: string;
  kind: StoredKind;
  nonce: string;
  ciphertext: string;
  hasFile: boolean;
  updatedAt: string;
}

export interface PasskeyRow {
  id: string;
  label: string;
  added: string;
  lastUsed: string | null;
}

export interface PasskeyList {
  passkeys: PasskeyRow[];
  enabled: boolean;
  userId: string;
}

export const api = {
  status: () => request<Status>("GET", "/api/status"),
  gate: (typed: string) => request<{ open: boolean }>("POST", "/api/gate", { typed }),
  prelogin: (username: string) =>
    request<{ kdf: KdfParams; kdfSalt: string }>("GET", `/api/prelogin?username=${encodeURIComponent(username)}`),
  signup: (username: string, enrollment: Enrollment) =>
    request<{ username: string }>("POST", "/api/signup", { username, enrollment }),
  login: (username: string, authKey: string) => request<{ username: string }>("POST", "/api/login", { username, authKey }),
  logout: () => request<void>("POST", "/api/logout", undefined, undefined, true),
  unlockStart: () => request<StepResponse>("POST", "/api/unlock/start"),
  answer: (step: number, proof: string) =>
    request<StepResponse | FinalResponse>("POST", "/api/unlock/answer", { step, proof }),
  lock: () => request<void>("POST", "/api/lock"),
  rekey: (currentAuthKey: string, enrollment: Enrollment) =>
    request<void>("PUT", "/api/account/keys", { currentAuthKey, enrollment }),
  passkeys: () => request<PasskeyList>("GET", "/api/passkeys"),
  passkeyStart: () => request<Record<string, unknown>>("POST", "/api/passkeys/start"),
  passkeyFinish: (label: string, credential: unknown) => request<PasskeyList>("POST", `/api/passkeys/finish?label=${encodeURIComponent(label)}`, credential),
  passkeyRename: (id: string, label: string) => request<PasskeyList>("PATCH", `/api/passkeys/${id}`, { label }),
  passkeyForget: (id: string) => request<PasskeyList>("DELETE", `/api/passkeys/${id}`),
  gatePasskeyStart: () => request<Record<string, unknown>>("POST", "/api/gate/passkey/start"),
  gatePasskeyFinish: (credential: unknown) => request<{ open: boolean }>("POST", "/api/gate/passkey/finish", credential),
  items: () => request<{ items: StoredItem[] }>("GET", "/api/items"),
  putItem: (id: string, kind: StoredKind, nonce: string, ciphertext: string, replaces?: string) =>
    request<void>("PUT", `/api/items/${id}`, { kind, nonce, ciphertext, replaces }),
  deleteItem: (id: string) => request<void>("DELETE", `/api/items/${id}`),
  putFile: (id: string, blob: Uint8Array) => request<void>("PUT", `/api/items/${id}/file`, undefined, blob),
  getFile: (id: string) => request<Uint8Array>("GET", `/api/items/${id}/file`),
};
