import { buf, concat, fromBase64, fromBase64Url, randomBytes, toBase64, utf8 } from "../crypto/encoding";
import { sshMpint, sshString, WireReader } from "./wire";

const MAGIC = utf8("openssh-key-v1\0");
const PEM_LINE = 70;

export type KeyAlgorithm = "ssh-ed25519" | "ssh-rsa";

export interface Ed25519Parts {
  kind: "ed25519";
  seed: Uint8Array;
  publicKey: Uint8Array;
}

export interface RsaParts {
  kind: "rsa";
  n: Uint8Array;
  e: Uint8Array;
  d: Uint8Array;
  p: Uint8Array;
  q: Uint8Array;
  iqmp: Uint8Array;
}

export type KeyParts = Ed25519Parts | RsaParts;

export interface PublicKeyInfo {
  algorithm: KeyAlgorithm;
  blob: Uint8Array;
  label: string;
}

export function publicBlob(parts: KeyParts): Uint8Array {
  if (parts.kind === "ed25519") return concat(sshString("ssh-ed25519"), sshString(parts.publicKey));
  return concat(sshString("ssh-rsa"), sshMpint(parts.e), sshMpint(parts.n));
}

export function encodePrivateKey(parts: KeyParts, comment: string): string {
  const check = randomBytes(4);
  const body =
    parts.kind === "ed25519"
      ? concat(
          sshString("ssh-ed25519"),
          sshString(parts.publicKey),
          sshString(concat(parts.seed, parts.publicKey)),
        )
      : concat(
          sshString("ssh-rsa"),
          sshMpint(parts.n),
          sshMpint(parts.e),
          sshMpint(parts.d),
          sshMpint(parts.iqmp),
          sshMpint(parts.p),
          sshMpint(parts.q),
        );

  let section = concat(check, check, body, sshString(comment));
  const pad = (8 - (section.length % 8)) % 8;
  section = concat(section, Uint8Array.from({ length: pad }, (_, i) => i + 1));

  const file = concat(
    MAGIC,
    sshString("none"),
    sshString("none"),
    sshString(""),
    new Uint8Array([0, 0, 0, 1]),
    sshString(publicBlob(parts)),
    sshString(section),
  );

  const b64 = toBase64(file).match(new RegExp(`.{1,${PEM_LINE}}`, "g"))!.join("\n");
  return `-----BEGIN OPENSSH PRIVATE KEY-----\n${b64}\n-----END OPENSSH PRIVATE KEY-----\n`;
}

export function publicKeyLine(info: PublicKeyInfo, comment: string): string {
  return `${info.algorithm} ${toBase64(info.blob)}${comment ? ` ${comment}` : ""}`;
}

export async function fingerprint(blob: Uint8Array): Promise<string> {
  const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", buf(blob)));
  return `SHA256:${toBase64(digest).replace(/=+$/, "")}`;
}

function describeBlob(blob: Uint8Array): PublicKeyInfo {
  const r = new WireReader(blob);
  const algorithm = new TextDecoder().decode(r.string());
  if (algorithm === "ssh-ed25519") return { algorithm, blob, label: "ed25519" };
  if (algorithm === "ssh-rsa") {
    r.string();
    const n = r.string();
    let bits = n.length * 8;
    for (let i = 0; i < n.length && n[i] === 0; i++) bits -= 8;
    return { algorithm, blob, label: `rsa-${Math.round(bits / 1024) * 1024}` };
  }
  throw new Error(`Unsupported key type ${algorithm}`);
}

export function parsePublicKeyLine(line: string): PublicKeyInfo {
  const [, b64] = line.trim().split(/\s+/);
  if (!b64) throw new Error("Not an SSH public key");
  return describeBlob(fromBase64(b64));
}

function pemBody(pem: string, label: string): Uint8Array | null {
  const m = pem.match(new RegExp(`-----BEGIN ${label}-----([\\s\\S]*?)-----END ${label}-----`));
  if (!m) return null;
  if (/Proc-Type:\s*4,ENCRYPTED/.test(m[1])) {
    throw new Error("This key is passphrase-encrypted PEM; paste its public key as well");
  }
  return fromBase64(m[1].replace(/^[A-Za-z-]+:.*$/gm, ""));
}

function derIntegers(der: Uint8Array, count: number): Uint8Array[] {
  let i = 0;
  const length = (): number => {
    const first = der[i++];
    if (first < 0x80) return first;
    let n = 0;
    for (let k = 0; k < (first & 0x7f); k++) n = (n << 8) | der[i++];
    return n;
  };
  if (der[i++] !== 0x30) throw new Error("Malformed RSA key");
  length();
  const out: Uint8Array[] = [];
  while (out.length < count) {
    if (der[i++] !== 0x02) throw new Error("Malformed RSA key");
    const n = length();
    out.push(der.subarray(i, i + n));
    i += n;
  }
  return out;
}

async function pkcs8PublicBlob(der: Uint8Array): Promise<Uint8Array> {
  try {
    const key = await crypto.subtle.importKey("pkcs8", buf(der), { name: "Ed25519" }, true, ["sign"]);
    const jwk = await crypto.subtle.exportKey("jwk", key);
    return concat(sshString("ssh-ed25519"), sshString(fromBase64Url(jwk.x!)));
  } catch {
    const key = await crypto.subtle.importKey(
      "pkcs8",
      buf(der),
      { name: "RSASSA-PKCS1-v1_5", hash: "SHA-256" },
      true,
      ["sign"],
    );
    const jwk = await crypto.subtle.exportKey("jwk", key);
    return concat(sshString("ssh-rsa"), sshMpint(fromBase64Url(jwk.e!)), sshMpint(fromBase64Url(jwk.n!)));
  }
}

export async function publicKeyFromPrivate(pem: string): Promise<PublicKeyInfo> {
  const openssh = pemBody(pem, "OPENSSH PRIVATE KEY");
  if (openssh) {
    if (!MAGIC.every((b, i) => openssh[i] === b)) throw new Error("Not an OpenSSH private key");
    const r = new WireReader(openssh.subarray(MAGIC.length));
    r.string();
    r.string();
    r.string();
    if (r.u32() !== 1) throw new Error("Key files holding several keys are not supported");
    return describeBlob(r.string());
  }

  const pkcs1 = pemBody(pem, "RSA PRIVATE KEY");
  if (pkcs1) {
    const [, n, e] = derIntegers(pkcs1, 3);
    return describeBlob(concat(sshString("ssh-rsa"), sshMpint(e), sshMpint(n)));
  }

  const pkcs8 = pemBody(pem, "PRIVATE KEY");
  if (pkcs8) return describeBlob(await pkcs8PublicBlob(pkcs8));

  if (pem.includes("ENCRYPTED PRIVATE KEY")) {
    throw new Error("This key is passphrase-encrypted PKCS#8; paste its public key as well");
  }
  throw new Error("Unrecognised private key format");
}

export async function describeKey(privateKey?: string, publicKey?: string) {
  if (!privateKey?.trim() && !publicKey?.trim()) return {};
  const info = publicKey?.trim() ? parsePublicKeyLine(publicKey) : await publicKeyFromPrivate(privateKey!);
  return {
    publicKey: publicKey?.trim() || publicKeyLine(info, ""),
    fingerprint: await fingerprint(info.blob),
    keyType: info.label,
  };
}
