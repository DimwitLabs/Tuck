import { argon2id } from "hash-wasm";

import { buf, concat, fromBase64, fromUtf8, randomBytes, toBase64, u32, utf8 } from "./encoding";

export interface KdfParams {
  algorithm: "argon2id";
  iterations: number;
  memoryKiB: number;
  parallelism: number;
}

export const DEFAULT_KDF: KdfParams = { algorithm: "argon2id", iterations: 3, memoryKiB: 65536, parallelism: 1 };

export const QUESTION_COUNT = 3;

export function checkKdf(kdf: KdfParams): void {
  const within = (n: number, lo: number, hi: number) => Number.isInteger(n) && n >= lo && n <= hi;
  if (
    kdf.algorithm !== "argon2id" ||
    !within(kdf.iterations, DEFAULT_KDF.iterations, 10) ||
    !within(kdf.memoryKiB, DEFAULT_KDF.memoryKiB, 1 << 20) ||
    !within(kdf.parallelism, 1, 4)
  ) {
    throw new Error("the server asked for key settings tuck doesn't accept, so nothing was sent.");
  }
}

async function stretch(secret: Uint8Array, salt: Uint8Array, kdf: KdfParams): Promise<Uint8Array> {
  checkKdf(kdf);
  return argon2id({
    password: secret,
    salt,
    iterations: kdf.iterations,
    memorySize: kdf.memoryKiB,
    parallelism: kdf.parallelism,
    hashLength: 32,
    outputType: "binary",
  });
}

async function hkdf(ikm: Uint8Array, info: string): Promise<Uint8Array> {
  const key = await crypto.subtle.importKey("raw", buf(ikm), "HKDF", false, ["deriveBits"]);
  const bits = await crypto.subtle.deriveBits(
    { name: "HKDF", hash: "SHA-256", salt: new Uint8Array(), info: buf(utf8(`tuck:${info}:v1`)) },
    key,
    256,
  );
  return new Uint8Array(bits);
}

async function aesKey(raw: Uint8Array): Promise<CryptoKey> {
  return crypto.subtle.importKey("raw", buf(raw), "AES-GCM", false, ["encrypt", "decrypt"]);
}

async function seal(key: CryptoKey, plaintext: Uint8Array, aad: string) {
  const nonce = randomBytes(12);
  const ciphertext = new Uint8Array(
    await crypto.subtle.encrypt({ name: "AES-GCM", iv: buf(nonce), additionalData: buf(utf8(aad)) }, key, buf(plaintext)),
  );
  return { nonce, ciphertext };
}

async function open(key: CryptoKey, nonce: Uint8Array, ciphertext: Uint8Array, aad: string): Promise<Uint8Array> {
  return new Uint8Array(
    await crypto.subtle.decrypt({ name: "AES-GCM", iv: buf(nonce), additionalData: buf(utf8(aad)) }, key, buf(ciphertext)),
  );
}

export function normalizeAnswer(answer: string): string {
  return answer.normalize("NFKC").trim().replace(/\s+/g, " ").toLocaleLowerCase("en-US");
}

const link = async (previous: Uint8Array, answer: string, salt: Uint8Array, kdf: KdfParams) => {
  const bytes = utf8(normalizeAnswer(answer));
  return stretch(concat(previous, u32(bytes.length), bytes), salt, kdf);
};

const questionKey = async (chain: Uint8Array, step: number) => aesKey(await hkdf(chain, `question:${step}`));
const proofFor = async (chain: Uint8Array, step: number) => toBase64(await hkdf(chain, `proof:${step}`));
const VAULT_AAD = "tuck:vault-key:v1";

export const deriveRoot = (password: string, kdfSalt: Uint8Array, kdf: KdfParams) =>
  stretch(utf8(password.normalize("NFKC")), kdfSalt, kdf);

export const deriveAuthKey = async (root: Uint8Array) => toBase64(await hkdf(root, "auth"));

export interface StepPublic {
  salt: string;
  questionNonce: string;
  questionCiphertext: string;
}

export interface EnrollmentStep extends StepPublic {
  proof: string;
}

export interface Keyring {
  kdf: KdfParams;
  kdfSalt: string;
  steps: StepPublic[];
  wrappedKey: string;
  wrappedNonce: string;
}

export const keyringOf = (e: Enrollment): Keyring => ({
  kdf: e.kdf,
  kdfSalt: e.kdfSalt,
  steps: e.steps.map(({ salt, questionNonce, questionCiphertext }) => ({ salt, questionNonce, questionCiphertext })),
  wrappedKey: e.wrappedKey,
  wrappedNonce: e.wrappedNonce,
});

const readStep = async (chain: Uint8Array, n: number, step: StepPublic) =>
  fromUtf8(await open(await questionKey(chain, n), fromBase64(step.questionNonce), fromBase64(step.questionCiphertext), `tuck:question:v1:${n}`));

export async function checkKeyring(
  k: Keyring,
  password: string,
  answers: string[],
): Promise<{ authKey: string; wrong: number | null; vaultKey?: Uint8Array }> {
  let chain = await deriveRoot(password, fromBase64(k.kdfSalt), k.kdf);
  const authKey = await deriveAuthKey(chain);
  for (let i = 0; i < QUESTION_COUNT; i++) {
    try {
      await readStep(chain, i + 1, k.steps[i]);
    } catch {
      return { authKey, wrong: i };
    }
    chain = await link(chain, answers[i], fromBase64(k.steps[i].salt), k.kdf);
  }
  try {
    const vaultKey = await open(await aesKey(await hkdf(chain, "kek")), fromBase64(k.wrappedNonce), fromBase64(k.wrappedKey), VAULT_AAD);
    return { authKey, wrong: null, vaultKey };
  } catch {
    return { authKey, wrong: QUESTION_COUNT };
  } finally {
    chain.fill(0);
  }
}

export interface Enrollment {
  kdf: KdfParams;
  kdfSalt: string;
  authKey: string;
  steps: EnrollmentStep[];
  wrappedKey: string;
  wrappedNonce: string;
}

export async function enroll(
  password: string,
  questions: string[],
  answers: string[],
  vaultKey: Uint8Array = randomBytes(32),
  kdf: KdfParams = DEFAULT_KDF,
): Promise<Enrollment> {
  if (questions.length !== QUESTION_COUNT || answers.length !== QUESTION_COUNT) {
    throw new Error(`Exactly ${QUESTION_COUNT} questions and answers are required`);
  }
  const kdfSalt = randomBytes(16);
  let chain = await deriveRoot(password, kdfSalt, kdf);
  const authKey = await deriveAuthKey(chain);

  const steps: EnrollmentStep[] = [];
  for (let i = 0; i < QUESTION_COUNT; i++) {
    const step = i + 1;
    const q = await seal(await questionKey(chain, step), utf8(questions[i].trim()), `tuck:question:v1:${step}`);
    const salt = randomBytes(16);
    chain = await link(chain, answers[i], salt, kdf);
    steps.push({
      salt: toBase64(salt),
      questionNonce: toBase64(q.nonce),
      questionCiphertext: toBase64(q.ciphertext),
      proof: await proofFor(chain, step),
    });
  }

  const wrapped = await seal(await aesKey(await hkdf(chain, "kek")), vaultKey, VAULT_AAD);
  return {
    kdf,
    kdfSalt: toBase64(kdfSalt),
    authKey,
    steps,
    wrappedKey: toBase64(wrapped.ciphertext),
    wrappedNonce: toBase64(wrapped.nonce),
  };
}

export class Unlocker {
  private step = 0;
  private readonly seen: StepPublic[] = [];
  private constructor(
    private chain: Uint8Array,
    private readonly kdf: KdfParams,
    private readonly kdfSalt: string,
  ) {}

  static async start(password: string, kdfSalt: string, kdf: KdfParams) {
    const root = await deriveRoot(password, fromBase64(kdfSalt), kdf);
    return { unlocker: new Unlocker(root, kdf, kdfSalt), authKey: await deriveAuthKey(root) };
  }

  get nextStep(): number {
    return this.step + 1;
  }

  readQuestion(step: StepPublic): Promise<string> {
    return readStep(this.chain, this.nextStep, step);
  }

  async attempt(answer: string, step: StepPublic) {
    const next = await link(this.chain, answer, fromBase64(step.salt), this.kdf);
    return { proof: await proofFor(next, this.nextStep), next };
  }

  accept(next: Uint8Array, step: StepPublic): void {
    this.chain.fill(0);
    this.chain = next;
    this.seen.push({ salt: step.salt, questionNonce: step.questionNonce, questionCiphertext: step.questionCiphertext });
    this.step++;
  }

  async finish(wrappedKey: string, wrappedNonce: string): Promise<{ vault: Vault; keyring: Keyring }> {
    if (this.step !== QUESTION_COUNT) throw new Error("All three questions must be answered first");
    const kek = await aesKey(await hkdf(this.chain, "kek"));
    const raw = await open(kek, fromBase64(wrappedNonce), fromBase64(wrappedKey), VAULT_AAD);
    const keyring = { kdf: this.kdf, kdfSalt: this.kdfSalt, steps: [...this.seen], wrappedKey, wrappedNonce };
    const vault = await Vault.fromRaw(raw);
    this.chain.fill(0);
    return { vault, keyring };
  }
}

export class Vault {
  private constructor(private readonly key: CryptoKey) {}

  static async fromRaw(raw: Uint8Array): Promise<Vault> {
    const vault = new Vault(await aesKey(raw));
    raw.fill(0);
    return vault;
  }

  async encryptItem(id: string, kind: string, payload: unknown) {
    const { nonce, ciphertext } = await seal(this.key, utf8(JSON.stringify(payload)), `tuck:item:v1:${kind}:${id}`);
    return { nonce: toBase64(nonce), ciphertext: toBase64(ciphertext) };
  }

  async decryptItem<T>(id: string, kind: string, nonce: string, ciphertext: string): Promise<T> {
    const plain = await open(this.key, fromBase64(nonce), fromBase64(ciphertext), `tuck:item:v1:${kind}:${id}`);
    return JSON.parse(fromUtf8(plain)) as T;
  }

  async encryptFile(id: string, bytes: Uint8Array): Promise<Uint8Array> {
    const { nonce, ciphertext } = await seal(this.key, bytes, `tuck:file:v1:${id}`);
    return concat(nonce, ciphertext);
  }

  async decryptFile(id: string, blob: Uint8Array): Promise<Uint8Array> {
    return open(this.key, blob.subarray(0, 12), blob.subarray(12), `tuck:file:v1:${id}`);
  }
}
