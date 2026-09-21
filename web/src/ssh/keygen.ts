import { fromBase64Url } from "../crypto/encoding";
import { encodePrivateKey, fingerprint, publicBlob, publicKeyLine, type KeyParts } from "./openssh";

export type KeySpec = "ed25519" | "rsa-2048" | "rsa-3072" | "rsa-4096";

export interface GeneratedKey {
  privateKey: string;
  publicKey: string;
  fingerprint: string;
  keyType: KeySpec;
}

async function ed25519Parts(): Promise<KeyParts> {
  const pair = (await crypto.subtle.generateKey({ name: "Ed25519" }, true, ["sign", "verify"])) as CryptoKeyPair;
  const jwk = await crypto.subtle.exportKey("jwk", pair.privateKey);
  return { kind: "ed25519", seed: fromBase64Url(jwk.d!), publicKey: fromBase64Url(jwk.x!) };
}

async function rsaParts(bits: number): Promise<KeyParts> {
  const pair = await crypto.subtle.generateKey(
    {
      name: "RSASSA-PKCS1-v1_5",
      modulusLength: bits,
      publicExponent: new Uint8Array([1, 0, 1]),
      hash: "SHA-256",
    },
    true,
    ["sign", "verify"],
  );
  const jwk = await crypto.subtle.exportKey("jwk", pair.privateKey);
  return {
    kind: "rsa",
    n: fromBase64Url(jwk.n!),
    e: fromBase64Url(jwk.e!),
    d: fromBase64Url(jwk.d!),
    p: fromBase64Url(jwk.p!),
    q: fromBase64Url(jwk.q!),
    iqmp: fromBase64Url(jwk.qi!),
  };
}

export async function generateKey(spec: KeySpec, comment: string): Promise<GeneratedKey> {
  const parts = spec === "ed25519" ? await ed25519Parts() : await rsaParts(Number(spec.split("-")[1]));
  const blob = publicBlob(parts);
  const algorithm = parts.kind === "ed25519" ? "ssh-ed25519" : "ssh-rsa";
  return {
    privateKey: encodePrivateKey(parts, comment),
    publicKey: publicKeyLine({ algorithm, blob, label: spec }, comment),
    fingerprint: await fingerprint(blob),
    keyType: spec,
  };
}
