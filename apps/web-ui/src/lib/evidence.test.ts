import { promises as fs } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { afterEach, describe, expect, it } from 'vitest';
import { MC_V2_STATUS } from './doctrine';
import { loadEvidence, parseCandidate, resolveEvidenceDir } from './evidence';

describe('resolveEvidenceDir', () => {
  afterEach(() => {
    delete process.env.EVIDENCE_DIR;
  });

  it('defaults to /evidence when EVIDENCE_DIR is unset', () => {
    delete process.env.EVIDENCE_DIR;
    expect(resolveEvidenceDir()).toBe('/evidence');
  });

  it('honors EVIDENCE_DIR when set', () => {
    process.env.EVIDENCE_DIR = '/tmp/some-evidence';
    expect(resolveEvidenceDir()).toBe('/tmp/some-evidence');
  });
});

describe('loadEvidence', () => {
  afterEach(() => {
    delete process.env.EVIDENCE_DIR;
  });

  it('returns UNKNOWN when EVIDENCE_DIR points at an absent directory', async () => {
    process.env.EVIDENCE_DIR = path.join(os.tmpdir(), 'omega-evidence-absent-xyz');
    const payload = await loadEvidence();
    expect(payload.available).toBe(false);
    expect(payload.status).toBe('UNKNOWN');
    expect(payload.campaigns).toEqual([]);
    expect(payload.configs).toEqual([]);
  });

  it('returns UNKNOWN when EVIDENCE_DIR points at a file, not a directory', async () => {
    const file = path.join(os.tmpdir(), `omega-evidence-file-${process.pid}.txt`);
    await fs.writeFile(file, 'not a directory');
    process.env.EVIDENCE_DIR = file;
    try {
      const payload = await loadEvidence();
      expect(payload.available).toBe(false);
      expect(payload.status).toBe('UNKNOWN');
    } finally {
      await fs.unlink(file);
    }
  });

  it('builds funnel, campaign records, and per-config detail from real artifacts', async () => {
    const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'omega-evidence-'));
    process.env.EVIDENCE_DIR = dir;
    try {
      await fs.writeFile(
        path.join(dir, 'campaign_summary.json'),
        JSON.stringify({ configurations_evaluated: 2, qualifier: null })
      );
      await fs.writeFile(
        path.join(dir, 'campaign2_summary.json'),
        JSON.stringify({ configurations_evaluated: 1, qualifier: null })
      );
      await fs.mkdir(path.join(dir, 'reports-c1'));
      await fs.mkdir(path.join(dir, 'reports-c2'));
      await fs.writeFile(
        path.join(dir, 'reports-c1', '01-sma-trend-fast12-slow48.json'),
        JSON.stringify({ candidate: '01-sma-trend-fast12-slow48', verdict: 'FAIL', failed_stage: 'costs' })
      );
      await fs.writeFile(
        path.join(dir, 'reports-c1', '10-donchian-window96.json'),
        JSON.stringify({ candidate: '10-donchian-window96', verdict: 'FAIL', failed_stage: 'walkforward' })
      );
      // Corroboration rows must not enter the funnel.
      await fs.writeFile(
        path.join(dir, 'reports-c1', '01-sma-trend-fast12-slow48-coinbase.json'),
        JSON.stringify({ candidate: '01-sma-trend-fast12-slow48', verdict: 'FAIL', failed_stage: 'costs' })
      );
      await fs.writeFile(
        path.join(dir, 'reports-c2', '56-day-of-week-dow0-n3.json'),
        JSON.stringify({ candidate: '56-day-of-week-dow0-n3', verdict: 'FAIL', failed_stage: 'montecarlo' })
      );

      const payload = await loadEvidence();

      expect(payload.available).toBe(true);
      expect(payload.status).toBe('OK');
      expect(payload.mcV2Status).toBe(MC_V2_STATUS);

      expect(payload.funnel.costs).toBe(1);
      expect(payload.funnel.walkforward).toBe(1);
      expect(payload.funnel.nulls).toBe(0);
      expect(payload.funnel.montecarlo).toBe(1);
      expect(payload.funnel.overfit).toBe(0);

      expect(payload.campaigns).toHaveLength(2);
      expect(payload.campaigns[0]).toMatchObject({
        campaign: 1,
        configsEvaluated: 2,
        verdict: 'FALSIFIED',
        qualifier: null
      });
      expect(payload.campaigns[0].funnel.costs).toBe(1);
      expect(payload.campaigns[1]).toMatchObject({ campaign: 2, configsEvaluated: 1 });

      expect(payload.configs).toHaveLength(3);
      expect(payload.configs[0]).toMatchObject({
        campaign: 1,
        number: 1,
        family: 'sma-trend',
        verdict: 'FAIL',
        stageReached: 'costs'
      });
      expect(payload.configs[2]).toMatchObject({
        campaign: 2,
        number: 56,
        family: 'day-of-week',
        stageReached: 'montecarlo'
      });
    } finally {
      await fs.rm(dir, { recursive: true, force: true });
      delete process.env.EVIDENCE_DIR;
    }
  });
});

  it('still loads reports containing non-standard NaN/Infinity tokens (lab artifact)', async () => {
    const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'omega-evidence-nan-'));
    process.env.EVIDENCE_DIR = dir;
    try {
      await fs.mkdir(path.join(dir, 'reports-c2'));
      // Mirrors the real #56 report: Python json emitted bare NaN/Infinity tokens.
      await fs.writeFile(
        path.join(dir, 'reports-c2', '56-day-of-week-dow0-n3.json'),
        '{"candidate": "56-day-of-week-dow0-n3", "verdict": "FAIL", "failed_stage": "montecarlo", "stages": {"montecarlo": {"metrics": {"p5": NaN, "median": Infinity, "neg": -Infinity}}}}'
      );
      const payload = await loadEvidence();
      expect(payload.available).toBe(true);
      expect(payload.funnel.montecarlo).toBe(1);
      expect(payload.configs).toHaveLength(1);
      expect(payload.configs[0]).toMatchObject({
        number: 56,
        family: 'day-of-week',
        stageReached: 'montecarlo'
      });
    } finally {
      await fs.rm(dir, { recursive: true, force: true });
      delete process.env.EVIDENCE_DIR;
    }
});

describe('parseCandidate', () => {
  it('extracts number and family from campaign candidate slugs', () => {
    expect(parseCandidate('01-sma-trend-fast12-slow48')).toEqual({ number: 1, family: 'sma-trend' });
    expect(parseCandidate('56-day-of-week-dow0-n3')).toEqual({ number: 56, family: 'day-of-week' });
    expect(parseCandidate('47-rv-regime-trend-t720-rv168-0.30-1.10')).toEqual({
      number: 47,
      family: 'rv-regime-trend'
    });
  });
});
