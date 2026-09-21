import { describe, expect, it } from "vitest";

import { randomBytes } from "./encoding";
import { checkKdf, checkKeyring, DEFAULT_KDF, enroll, keyringOf, normalizeAnswer, Unlocker, Vault, type Enrollment, type KdfParams } from "./vault";

const KDF: KdfParams = DEFAULT_KDF;
const ARGON2_TIMEOUT_MS = 60_000;
const questions = ["First pet?", "Street you grew up on?", "Favourite teacher?"];
const answers = ["Bruno", "Rajouri Garden", "Mrs. Sharma"];

async function walk(e: Enrollment, password: string, given: string[]) {
  const { unlocker, authKey } = await Unlocker.start(password, e.kdfSalt, e.kdf);
  const seen: string[] = [];
  for (let i = 0; i < 3; i++) {
    seen.push(await unlocker.readQuestion(e.steps[i]));
    const { proof, next } = await unlocker.attempt(given[i], e.steps[i]);
    if (proof !== e.steps[i].proof) return { authKey, seen, stoppedAt: i + 1 };
    unlocker.accept(next, e.steps[i]);
  }
  const { vault, keyring } = await unlocker.finish(e.wrappedKey, e.wrappedNonce);
  return { authKey, seen, vault, keyring };
}

describe("vault key chain", { timeout: ARGON2_TIMEOUT_MS }, () => {
  it("walks all three questions in order and opens the vault", async () => {
    const e = await enroll("correct horse", questions, answers, undefined, KDF);
    const r = await walk(e, "correct horse", answers);
    expect(r.authKey).toBe(e.authKey);
    expect(r.seen).toEqual(questions);
    expect(r.vault).toBeInstanceOf(Vault);
  });

  it("stops at the first wrong answer, and the next question stays unreadable", async () => {
    const e = await enroll("correct horse", questions, answers, undefined, KDF);
    const r = await walk(e, "correct horse", ["Bruno", "wrong street", "Mrs. Sharma"]);
    expect(r.stoppedAt).toBe(2);

    const { unlocker } = await Unlocker.start("correct horse", e.kdfSalt, e.kdf);
    const { next } = await unlocker.attempt("not bruno", e.steps[0]);
    unlocker.accept(next, e.steps[0]);
    await expect(unlocker.readQuestion(e.steps[1])).rejects.toThrow();
  });

  it("a wrong password yields a different auth key and cannot read question 1", async () => {
    const e = await enroll("correct horse", questions, answers, undefined, KDF);
    const { unlocker, authKey } = await Unlocker.start("wrong", e.kdfSalt, e.kdf);
    expect(authKey).not.toBe(e.authKey);
    await expect(unlocker.readQuestion(e.steps[0])).rejects.toThrow();
  });

  it("ignores case, spacing and unicode form, but not answer boundaries", async () => {
    const e = await enroll("pw", questions, answers, undefined, KDF);
    expect((await walk(e, "pw", ["  BRUNO ", "rajouri   garden", "MRS. SHARMA"])).vault).toBeDefined();
    expect(normalizeAnswer("Café")).toBe(normalizeAnswer("café"));
    expect((await walk(e, "pw", ["Brun", "oRajouri Garden", "Mrs. Sharma"])).stoppedAt).toBe(1);
  });

  it("refuses to finish before all three are accepted", async () => {
    const e = await enroll("pw", questions, answers, undefined, KDF);
    const { unlocker } = await Unlocker.start("pw", e.kdfSalt, e.kdf);
    await expect(unlocker.finish(e.wrappedKey, e.wrappedNonce)).rejects.toThrow(/All three/);
  });

  it("round-trips items and files, and binds them to their id and kind", async () => {
    const vault = await Vault.fromRaw(randomBytes(32));
    const sealed = await vault.encryptItem("id-1", "credential", { privateKey: "x" });
    expect(await vault.decryptItem("id-1", "credential", sealed.nonce, sealed.ciphertext)).toEqual({ privateKey: "x" });
    await expect(vault.decryptItem("id-2", "credential", sealed.nonce, sealed.ciphertext)).rejects.toThrow();
    await expect(vault.decryptItem("id-1", "host", sealed.nonce, sealed.ciphertext)).rejects.toThrow();
    const bytes = randomBytes(5000);
    expect(await vault.decryptFile("f", await vault.encryptFile("f", bytes))).toEqual(bytes);
  });

  it("rekeying keeps the vault key, so existing items still open", async () => {
    const e = await enroll("pw", questions, answers, undefined, KDF);
    const { vault } = await walk(e, "pw", answers);
    const sealed = await vault!.encryptItem("id-1", "host", { name: "kept" });

    const { vaultKey } = await checkKeyring(keyringOf(e), "pw", answers);
    const rekeyed = await enroll("new pw", ["a?", "b?", "c?"], ["1", "2", "3"], vaultKey, KDF);
    const reopened = (await walk(rekeyed, "new pw", ["1", "2", "3"])).vault!;
    expect(await reopened.decryptItem("id-1", "host", sealed.nonce, sealed.ciphertext)).toEqual({ name: "kept" });
    expect((await walk(rekeyed, "new pw", answers)).stoppedAt).toBe(1);
  });

  it("checks a password and answers locally, naming the first wrong one", async () => {
    const e = await enroll("pw", questions, answers, undefined, KDF);
    const k = keyringOf(e);
    expect(await checkKeyring(k, "pw", answers)).toMatchObject({ authKey: e.authKey, wrong: null });
    expect((await checkKeyring(k, "nope", answers)).wrong).toBe(0);
    expect((await checkKeyring(k, "pw", ["Bruno", "Rajouri Gardens", "Mrs. Sharma"])).wrong).toBe(2);
    expect((await checkKeyring(k, "pw", ["Bruno", "Rajouri Garden", "Mr. Sharma"])).wrong).toBe(3);
    expect((await walk(e, "pw", answers)).keyring).toEqual(k);
  });

  it("refuses key settings weaker than its own, whatever the server asks for", async () => {
    const e = await enroll("pw", questions, answers, undefined, KDF);
    for (const kdf of [
      { ...DEFAULT_KDF, iterations: 1 },
      { ...DEFAULT_KDF, memoryKiB: 8 },
      { ...DEFAULT_KDF, memoryKiB: 1 << 30 },
      { ...DEFAULT_KDF, algorithm: "pbkdf2" },
    ]) {
      await expect(Unlocker.start("pw", e.kdfSalt, kdf as KdfParams)).rejects.toThrow(/key settings/);
      expect(() => checkKdf(kdf as KdfParams)).toThrow();
    }
  });
});
