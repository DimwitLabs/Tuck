const toBuffer = (s: string) => {
  const b64 = s.replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(b64.padEnd(b64.length + ((4 - (b64.length % 4)) % 4), "="));
  return Uint8Array.from(raw, (c) => c.charCodeAt(0));
};

const toText = (b: ArrayBuffer) =>
  btoa(String.fromCharCode(...new Uint8Array(b)))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");

type Descriptor = { id: string; type: string; transports?: string[] };
type CreationJSON = { challenge: string; user: { id: string }; excludeCredentials?: Descriptor[] };
type RequestJSON = { challenge: string; allowCredentials?: Descriptor[] };

const descriptors = (list: Descriptor[] | undefined) => list?.map((d) => ({ ...d, id: toBuffer(d.id) }));

// parseCreationOptionsFromJSON and toJSON only reached Safari in 18.4, so both paths stay.
export function creationOptions(json: CreationJSON): PublicKeyCredentialCreationOptions {
  if (PublicKeyCredential.parseCreationOptionsFromJSON) {
    return PublicKeyCredential.parseCreationOptionsFromJSON(json as never);
  }
  const options = json as unknown as PublicKeyCredentialCreationOptions;
  return { ...options, challenge: toBuffer(json.challenge), user: { ...options.user, id: toBuffer(json.user.id) }, excludeCredentials: descriptors(json.excludeCredentials) as never };
}

export function requestOptions(json: RequestJSON): PublicKeyCredentialRequestOptions {
  if (PublicKeyCredential.parseRequestOptionsFromJSON) {
    return PublicKeyCredential.parseRequestOptionsFromJSON(json as never);
  }
  const options = json as unknown as PublicKeyCredentialRequestOptions;
  return { ...options, challenge: toBuffer(json.challenge), allowCredentials: descriptors(json.allowCredentials) as never };
}

export function credentialJSON(credential: PublicKeyCredential): unknown {
  if (typeof credential.toJSON === "function") return credential.toJSON();
  const response = credential.response;
  const shared = { id: credential.id, rawId: toText(credential.rawId), type: credential.type, clientExtensionResults: credential.getClientExtensionResults() };
  if (response instanceof AuthenticatorAttestationResponse) {
    return {
      ...shared,
      response: {
        clientDataJSON: toText(response.clientDataJSON),
        attestationObject: toText(response.attestationObject),
        transports: response.getTransports?.() ?? [],
      },
    };
  }
  const assertion = response as AuthenticatorAssertionResponse;
  return {
    ...shared,
    response: {
      clientDataJSON: toText(assertion.clientDataJSON),
      authenticatorData: toText(assertion.authenticatorData),
      signature: toText(assertion.signature),
      userHandle: assertion.userHandle ? toText(assertion.userHandle) : null,
    },
  };
}

// The door must look blank to a stranger, so only a browser that enrolled here knows to ask for a passkey at all.
const ENROLLED = "tuck.device";

export const rememberDevice = () => {
  try {
    localStorage.setItem(ENROLLED, "1");
  } catch {
    // storage blocked: the long press still works
  }
};

export const forgetDevice = () => {
  try {
    localStorage.removeItem(ENROLLED);
  } catch {
    // nothing to forget
  }
};

export const deviceEnrolled = () => {
  try {
    return localStorage.getItem(ENROLLED) === "1";
  } catch {
    return false;
  }
};

const DEVICES: [RegExp, string][] = [
  [/iphone/i, "this iphone"],
  [/ipad/i, "this ipad"],
  [/macintosh|mac os/i, "this mac"],
  [/android/i, "this android device"],
  [/windows/i, "this windows pc"],
  [/linux|cros/i, "this computer"],
];

export const deviceName = () => DEVICES.find(([pattern]) => pattern.test(navigator.userAgent))?.[1] ?? "this device";

export async function canEnrol(): Promise<boolean> {
  if (typeof PublicKeyCredential === "undefined" || !window.isSecureContext) return false;
  if (PublicKeyCredential.getClientCapabilities) {
    const caps = await PublicKeyCredential.getClientCapabilities();
    return !!caps.passkeyPlatformAuthenticator;
  }
  return PublicKeyCredential.isUserVerifyingPlatformAuthenticatorAvailable();
}

// A cancelled dialog is not a failure worth a message; everything else is.
export function passkeyProblem(err: unknown): string | null {
  const name = err instanceof Error ? err.name : "";
  if (name === "NotAllowedError" || name === "AbortError") return null;
  if (name === "InvalidStateError") return "this device is already enrolled.";
  if (name === "SecurityError") return "this address doesn't match the one tuck is configured with.";
  if (name === "NotSupportedError") return "this device can't make the kind of passkey tuck asks for.";
  if (name === "ConstraintError") return "this device needs a screen lock before it can hold a passkey.";
  return name ? `that device could not be enrolled (${name}).` : "that device could not be enrolled.";
}
