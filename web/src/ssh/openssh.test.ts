import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { generateKeyPairSync } from "node:crypto";
import { afterAll, describe, expect, it } from "vitest";

import { generateKey, type KeySpec } from "./keygen";
import { fingerprint, parsePublicKeyLine, publicKeyFromPrivate, publicKeyLine } from "./openssh";

const dir = mkdtempSync(join(tmpdir(), "tuck-ssh-"));
afterAll(() => rmSync(dir, { recursive: true, force: true }));

const keygen = (...args: string[]) => execFileSync("ssh-keygen", args, { encoding: "utf8" }).trim();
const stripComment = (line: string) => line.trim().split(/\s+/).slice(0, 2).join(" ");

function write(name: string, contents: string): string {
  const path = join(dir, name);
  writeFileSync(path, contents, { mode: 0o600 });
  return path;
}

describe("generated keys are accepted by OpenSSH", () => {
  const specs: KeySpec[] = ["ed25519", "rsa-2048", "rsa-3072"];

  it.each(specs)("%s: public key, fingerprint, and a real signature all verify", async (spec) => {
    const key = await generateKey(spec, `tuck-test-${spec}`);
    const path = write(`gen-${spec}`, key.privateKey);

    expect(stripComment(keygen("-y", "-f", path))).toBe(stripComment(key.publicKey));
    expect(keygen("-l", "-f", path).split(" ")[1]).toBe(key.fingerprint);

    const data = write(`data-${spec}`, "tuck signs this");
    keygen("-Y", "sign", "-q", "-f", path, "-n", "tuck", data);
    const signers = write(`signers-${spec}`, `tester ${key.publicKey}\n`);
    const verified = execFileSync(
      "ssh-keygen",
      ["-Y", "verify", "-f", signers, "-I", "tester", "-n", "tuck", "-s", `${data}.sig`],
      { input: readFileSync(data), encoding: "utf8" },
    );
    expect(verified).toContain("Good");
  }, 60_000);
});

describe("public keys are recovered from pasted private keys", () => {
  const cases: [string, string[]][] = [
    ["openssh ed25519", ["-t", "ed25519"]],
    ["openssh rsa", ["-t", "rsa", "-b", "2048"]],
    ["pkcs1 rsa (BEGIN RSA PRIVATE KEY)", ["-t", "rsa", "-b", "2048", "-m", "PEM"]],
    ["pkcs8 rsa (BEGIN PRIVATE KEY)", ["-t", "rsa", "-b", "2048", "-m", "PKCS8"]],
    ["passphrase-protected openssh", ["-t", "ed25519", "-N", "hunter2"]],
  ];

  it.each(cases)("%s", async (name, args) => {
    const path = join(dir, name.replace(/\W+/g, "-"));
    const passphrase = args.includes("-N") ? [] : ["-N", ""];
    keygen("-q", ...args, ...passphrase, "-C", "import-test", "-f", path);

    const info = await publicKeyFromPrivate(readFileSync(path, "utf8"));
    const expected = readFileSync(`${path}.pub`, "utf8");
    expect(publicKeyLine(info, "")).toBe(stripComment(expected));
    expect(await fingerprint(info.blob)).toBe(keygen("-l", "-f", `${path}.pub`).split(" ")[1]);
    expect(parsePublicKeyLine(expected).label).toBe(info.label);
  });
});

// macOS ssh-keygen (LibreSSL) cannot write PKCS#8 Ed25519, so Node makes the file.
it("pkcs8 ed25519 (BEGIN PRIVATE KEY)", async () => {
  const { privateKey, publicKey } = generateKeyPairSync("ed25519");
  const pem = privateKey.export({ type: "pkcs8", format: "pem" }).toString();
  const x = Buffer.from(publicKey.export({ format: "jwk" }).x!, "base64url");
  const blob = Buffer.concat([Buffer.from([0, 0, 0, 11]), Buffer.from("ssh-ed25519"), Buffer.from([0, 0, 0, 32]), x]);

  const info = await publicKeyFromPrivate(pem);
  expect(publicKeyLine(info, "")).toBe(`ssh-ed25519 ${blob.toString("base64")}`);
  expect(info.label).toBe("ed25519");
});
