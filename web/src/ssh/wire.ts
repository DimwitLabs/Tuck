import { concat, u32, utf8 } from "../crypto/encoding";

export const sshString = (b: Uint8Array | string): Uint8Array => {
  const bytes = typeof b === "string" ? utf8(b) : b;
  return concat(u32(bytes.length), bytes);
};

export function sshMpint(magnitude: Uint8Array): Uint8Array {
  let start = 0;
  while (start < magnitude.length && magnitude[start] === 0) start++;
  const trimmed = magnitude.subarray(start);
  const body = trimmed.length && trimmed[0] & 0x80 ? concat(new Uint8Array([0]), trimmed) : trimmed;
  return sshString(body);
}

export class WireReader {
  private offset = 0;
  constructor(private readonly data: Uint8Array) {}

  u32(): number {
    if (this.offset + 4 > this.data.length) throw new Error("Truncated key data");
    const n = new DataView(this.data.buffer, this.data.byteOffset + this.offset, 4).getUint32(0);
    this.offset += 4;
    return n;
  }

  bytes(n: number): Uint8Array {
    if (this.offset + n > this.data.length) throw new Error("Truncated key data");
    const out = this.data.subarray(this.offset, this.offset + n);
    this.offset += n;
    return out;
  }

  string(): Uint8Array {
    return this.bytes(this.u32());
  }
}
