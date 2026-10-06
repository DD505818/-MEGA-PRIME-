import os from 'node:os';
import path from 'node:path';
import { afterEach, describe, expect, it } from 'vitest';
import { GET } from './route';

describe('GET /api/evidence', () => {
  afterEach(() => {
    delete process.env.EVIDENCE_DIR;
  });

  it('returns HTTP 200 with UNKNOWN when EVIDENCE_DIR is absent', async () => {
    process.env.EVIDENCE_DIR = path.join(os.tmpdir(), 'omega-evidence-route-absent-xyz');
    const res = await GET();
    expect(res.status).toBe(200);
    const body = (await res.json()) as { available: boolean; status: string };
    expect(body.available).toBe(false);
    expect(body.status).toBe('UNKNOWN');
  });
});
