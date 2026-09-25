import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { canEnrol, creationOptions, credentialJSON, deviceEnrolled, deviceName, forgetDevice, passkeyProblem, requestOptions, rememberDevice } from "./passkey";

const b64u = (s: string) => btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
const bytes = (b: ArrayBuffer) => String.fromCharCode(...new Uint8Array(b));

describe("options parsing", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  const json = {
    challenge: b64u("challenge"),
    user: { id: b64u("user-id"), name: "havi", displayName: "havi" },
    excludeCredentials: [{ id: b64u("known"), type: "public-key", transports: ["internal"] }],
  };

  it("hands the browser its own parser when it has one", () => {
    const parsed = { parsed: true };
    vi.stubGlobal("PublicKeyCredential", { parseCreationOptionsFromJSON: () => parsed, parseRequestOptionsFromJSON: () => parsed });
    expect(creationOptions(json as never)).toBe(parsed);
    expect(requestOptions({ challenge: json.challenge } as never)).toBe(parsed);
  });

  it("decodes base64url itself on browsers without one", () => {
    vi.stubGlobal("PublicKeyCredential", {});
    const options = creationOptions(json as never);
    expect(bytes(options.challenge as ArrayBuffer)).toBe("challenge");
    expect(bytes(options.user.id as ArrayBuffer)).toBe("user-id");
    expect(bytes(options.excludeCredentials![0].id as ArrayBuffer)).toBe("known");
    expect(options.excludeCredentials![0].transports).toEqual(["internal"]);

    const request = requestOptions({ challenge: json.challenge, allowCredentials: json.excludeCredentials } as never);
    expect(bytes(request.challenge as ArrayBuffer)).toBe("challenge");
    expect(bytes(request.allowCredentials![0].id as ArrayBuffer)).toBe("known");
  });

  it("leaves allowCredentials alone when the door names none", () => {
    vi.stubGlobal("PublicKeyCredential", {});
    expect(requestOptions({ challenge: json.challenge } as never).allowCredentials).toBeUndefined();
  });
});

describe("credentialJSON", () => {
  const buffer = (s: string) => Uint8Array.from(s, (c) => c.charCodeAt(0)).buffer;

  it("uses the browser's toJSON when there is one", () => {
    const credential = { toJSON: () => ({ done: true }) };
    expect(credentialJSON(credential as never)).toEqual({ done: true });
  });

  it("packs an attestation by hand otherwise", () => {
    vi.stubGlobal("AuthenticatorAttestationResponse", class {});
    const response = Object.create(AuthenticatorAttestationResponse.prototype);
    Object.assign(response, { clientDataJSON: buffer("client"), attestationObject: buffer("attestation"), getTransports: () => ["internal"] });
    const packed = credentialJSON({
      id: "abc",
      rawId: buffer("raw"),
      type: "public-key",
      response,
      getClientExtensionResults: () => ({}),
    } as never) as Record<string, never>;

    expect(packed.id).toBe("abc");
    expect(packed.rawId).toBe(b64u("raw"));
    expect(packed.response).toEqual({ clientDataJSON: b64u("client"), attestationObject: b64u("attestation"), transports: ["internal"] });
    vi.unstubAllGlobals();
  });

  it("packs an assertion by hand otherwise", () => {
    vi.stubGlobal("AuthenticatorAttestationResponse", class {});
    const packed = credentialJSON({
      id: "abc",
      rawId: buffer("raw"),
      type: "public-key",
      response: { clientDataJSON: buffer("client"), authenticatorData: buffer("auth"), signature: buffer("sig"), userHandle: buffer("who") },
      getClientExtensionResults: () => ({}),
    } as never) as Record<string, never>;

    expect(packed.response).toEqual({
      clientDataJSON: b64u("client"),
      authenticatorData: b64u("auth"),
      signature: b64u("sig"),
      userHandle: b64u("who"),
    });
    vi.unstubAllGlobals();
  });

  it("reports no user handle when the authenticator sent none", () => {
    vi.stubGlobal("AuthenticatorAttestationResponse", class {});
    const packed = credentialJSON({
      id: "abc",
      rawId: buffer("raw"),
      type: "public-key",
      response: { clientDataJSON: buffer("client"), authenticatorData: buffer("auth"), signature: buffer("sig"), userHandle: null },
      getClientExtensionResults: () => ({}),
    } as never) as { response: { userHandle: string | null } };

    expect(packed.response.userHandle).toBeNull();
    vi.unstubAllGlobals();
  });
});

describe("canEnrol", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("says no where there is no webauthn at all", async () => {
    vi.stubGlobal("PublicKeyCredential", undefined);
    vi.stubGlobal("window", { isSecureContext: true });
    expect(await canEnrol()).toBe(false);
  });

  it("says no over plain http", async () => {
    vi.stubGlobal("PublicKeyCredential", {});
    vi.stubGlobal("window", { isSecureContext: false });
    expect(await canEnrol()).toBe(false);
  });

  it("asks getClientCapabilities when the browser has it", async () => {
    vi.stubGlobal("window", { isSecureContext: true });
    vi.stubGlobal("PublicKeyCredential", { getClientCapabilities: async () => ({ passkeyPlatformAuthenticator: true }) });
    expect(await canEnrol()).toBe(true);

    vi.stubGlobal("PublicKeyCredential", { getClientCapabilities: async () => ({ passkeyPlatformAuthenticator: false }) });
    expect(await canEnrol()).toBe(false);
  });

  it("falls back to the older question", async () => {
    vi.stubGlobal("window", { isSecureContext: true });
    vi.stubGlobal("PublicKeyCredential", { isUserVerifyingPlatformAuthenticatorAvailable: async () => true });
    expect(await canEnrol()).toBe(true);
  });
});

describe("passkeyProblem", () => {
  it("stays quiet when the person just cancelled", () => {
    expect(passkeyProblem(Object.assign(new Error(), { name: "NotAllowedError" }))).toBeNull();
    expect(passkeyProblem(Object.assign(new Error(), { name: "AbortError" }))).toBeNull();
  });

  it("explains the ones worth explaining", () => {
    expect(passkeyProblem(Object.assign(new Error(), { name: "InvalidStateError" }))).toMatch(/already enrolled/);
    expect(passkeyProblem(Object.assign(new Error(), { name: "SecurityError" }))).toMatch(/address/);
    expect(passkeyProblem(new Error("who knows"))).toMatch(/could not be enrolled/);
  });
});

describe("this device", () => {
  const store = new Map<string, string>();

  beforeEach(() => {
    store.clear();
    vi.stubGlobal("localStorage", {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => void store.set(k, v),
      removeItem: (k: string) => void store.delete(k),
    });
  });

  afterEach(() => vi.unstubAllGlobals());

  it("remembers and forgets that this browser enrolled", () => {
    expect(deviceEnrolled()).toBe(false);
    rememberDevice();
    expect(deviceEnrolled()).toBe(true);
    forgetDevice();
    expect(deviceEnrolled()).toBe(false);
  });

  it("never throws when storage is blocked", () => {
    vi.stubGlobal("localStorage", {
      getItem: () => {
        throw new Error("blocked");
      },
      setItem: () => {
        throw new Error("blocked");
      },
      removeItem: () => {
        throw new Error("blocked");
      },
    });
    expect(() => rememberDevice()).not.toThrow();
    expect(() => forgetDevice()).not.toThrow();
    expect(deviceEnrolled()).toBe(false);
  });

  it("names the device after the machine it runs on", () => {
    for (const [ua, want] of [
      ["Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X)", "this iphone"],
      ["Mozilla/5.0 (iPad; CPU OS 18_0 like Mac OS X)", "this ipad"],
      ["Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)", "this mac"],
      ["Mozilla/5.0 (Linux; Android 15)", "this android device"],
      ["Mozilla/5.0 (Windows NT 10.0; Win64; x64)", "this windows pc"],
      ["Mozilla/5.0 (X11; Linux x86_64)", "this computer"],
      ["Mozilla/5.0 (Nintendo Switch)", "this device"],
    ] as const) {
      vi.stubGlobal("navigator", { userAgent: ua });
      expect(deviceName()).toBe(want);
    }
  });
});
