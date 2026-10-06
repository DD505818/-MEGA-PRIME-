import { describe, expect, it } from 'vitest';
import {
  AEGIS_GATES,
  AUTHORITY_CHAIN,
  LIVE_LOCKED_WORDING,
  LIVE_STATE,
  MC_V2_STATUS,
  PROGRAM_BANNER,
  PROGRAM_STATE
} from './doctrine';

describe('doctrine constants', () => {
  it('exposes the exact program state', () => {
    expect(PROGRAM_STATE).toBe('EDGE NOT PROVEN');
    expect(LIVE_STATE).toBe('LIVE LOCKED');
    expect(PROGRAM_BANNER).toBe('EDGE NOT PROVEN · LIVE LOCKED');
  });

  it('uses the exact required live-lock wording', () => {
    expect(LIVE_LOCKED_WORDING).toBe(
      'Requires independent certification and backend governance outside this interface.'
    );
  });

  it('keeps the exact authority chain order', () => {
    expect([...AUTHORITY_CHAIN]).toEqual([
      'DATA',
      'MODELS',
      'AGENTS',
      'MIDAS',
      'AEGIS',
      'VULTURE',
      'TRUTHCORE',
      'VALIDATION'
    ]);
  });

  it('lists exactly 14 AEGIS gates in order with exact reason codes', () => {
    expect(AEGIS_GATES).toHaveLength(14);
    expect(AEGIS_GATES.map((g) => g.n)).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14]);

    const byN = new Map(AEGIS_GATES.map((g) => [g.n, g]));
    expect(byN.get(1)?.reason).toBe('GATE1_KILL_SWITCH_ACTIVE');
    expect(byN.get(2)?.reason).toBe('GATE2_CIRCUIT_BREAKER_ACTIVE');
    expect(byN.get(3)?.reason).toContain('GATE3_NOT_IN_PAPER_MODE');
    expect(byN.get(3)?.reason).toContain('GATE3_SIGNAL_MODE_MISMATCH');
    expect(byN.get(4)?.reason).toBe('GATE4_BROKER_DOWN');
    expect(byN.get(5)?.reason).toContain('GATE5_NO_BOOK_TS');
    expect(byN.get(6)?.reason).toBe('GATE6_DAILY_LOSS_EXCEEDED');
    expect(byN.get(7)?.reason).toBe('GATE7_MAX_DRAWDOWN_EXCEEDED');
    expect(byN.get(8)?.reason).toBe('GATE8_MAX_POSITIONS_REACHED');
    expect(byN.get(9)?.reason).toContain('GATE9_ASSET_EXPOSURE');
    expect(byN.get(10)?.reason).toContain('GATE10_HIGH_CORRELATION');
    expect(byN.get(11)?.reason).toContain('GATE11_DUPLICATE_SIGNAL_ID');
    expect(byN.get(12)?.reason).toContain('GATE12_SPREAD_TOO_WIDE');
    expect(byN.get(13)?.reason).toContain('GATE13_LOW_CONFIDENCE');
    expect(byN.get(14)?.reason).toContain('GATE14_INVALID_PRICE');
    expect(byN.get(14)?.reason).toContain('GATE14_QTY_REDUCED_TO_ZERO');

    for (const gate of AEGIS_GATES) {
      expect(gate.name.length).toBeGreaterThan(0);
      expect(gate.purpose.length).toBeGreaterThan(0);
    }
  });

  it('flags MC v2 as in review and unmerged', () => {
    expect(MC_V2_STATUS).toBe('in review (PR #75, unmerged)');
  });
});
