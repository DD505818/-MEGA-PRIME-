/**
 * Evidence loader for the ΩMEGA PRIME Δ operator dashboard.
 *
 * Reads the frozen edge-campaign artifacts from a mounted evidence directory
 * and builds the kill funnel, campaign records, and per-configuration detail.
 * It never fabricates: if the evidence directory or its files are absent, the
 * payload reports available:false with status 'UNKNOWN'.
 */

import { promises as fs } from 'node:fs';
import path from 'node:path';
import { MC_V2_STATUS } from './doctrine';
// Client-safe types/constants live in evidence-types.ts (no Node imports);
// re-exported here so server-side importers (route, tests) keep one path.
import { FUNNEL_STAGES } from './evidence-types';
import type { ConfigEvidence, CampaignEvidence, EvidencePayload } from './evidence-types';
export { FUNNEL_STAGES };
export type { ConfigEvidence, CampaignEvidence, EvidencePayload };

/**
 * Expected mount layout under $EVIDENCE_DIR (default: /evidence):
 *   campaign_summary.json     — Campaign #1 summary (configs evaluated, budgets, qualifier)
 *   campaign2_summary.json    — Campaign #2 summary (configs evaluated, budgets, qualifier)
 *   reports-c1/*.json         — per-configuration lab reports, campaign #1
 *   reports-c2/*.json         — per-configuration lab reports, campaign #2
 *
 * Per-config reports carry { candidate, verdict, failed_stage }. Files whose
 * name contains 'coinbase' are cross-venue corroboration rows, not primary
 * campaign configurations, and are excluded from the funnel.
 */
export const DEFAULT_EVIDENCE_DIR = '/evidence';

export function resolveEvidenceDir(): string {
  return process.env.EVIDENCE_DIR ?? DEFAULT_EVIDENCE_DIR;
}

/**
 * The validation lab writes reports with Python's json module, which emits
 * non-standard NaN / Infinity tokens for degenerate fits (e.g. config #56's
 * Monte Carlo fat-tail fit on zero-inflated returns). Sanitize those tokens
 * to null so the report still loads — dropping the deepest campaign run
 * would silently corrupt the funnel.
 */
function sanitizeNonStandardJson(text: string): string {
  return text
    .replace(/(?<=[:\[,\s])NaN(?=[,\]\s}])/g, 'null')
    .replace(/(?<=[:\[,\s])-Infinity(?=[,\]\s}])/g, 'null')
    .replace(/(?<=[:\[,\s])Infinity(?=[,\]\s}])/g, 'null');
}

async function readJsonOptional<T>(file: string): Promise<T | null> {
  try {
    const text = await fs.readFile(file, 'utf8');
    try {
      return JSON.parse(text) as T;
    } catch {
      return JSON.parse(sanitizeNonStandardJson(text)) as T;
    }
  } catch {
    return null;
  }
}

async function listDirOptional(dir: string): Promise<string[]> {
  try {
    return await fs.readdir(dir);
  } catch {
    return [];
  }
}

/** Extracts the config number and family from a candidate slug like '56-day-of-week-dow0-n3'. */
export function parseCandidate(candidate: string): { number: number; family: string } {
  const m = /^(\d+)-(.*)$/.exec(candidate);
  const number = m ? parseInt(m[1], 10) : 0;
  const rest = m ? m[2] : candidate;
  // Family = leading dash-separated segments that contain no digits (parameter tokens carry digits).
  const family = rest
    .split('-')
    .filter((seg) => !/\d/.test(seg))
    .join('-');
  return { number, family: family || rest };
}

function emptyFunnel(): Record<string, number> {
  const f: Record<string, number> = {};
  for (const s of FUNNEL_STAGES) f[s] = 0;
  f['passed'] = 0;
  return f;
}

interface SummaryShape {
  configurations_evaluated?: number;
  qualifier?: string | null;
  verdict?: string;
}

async function loadCampaign(
  dir: string,
  campaign: number,
  summaryFile: string,
  reportsSubdir: string
): Promise<{ record: CampaignEvidence; configs: ConfigEvidence[]; found: boolean }> {
  const summary = await readJsonOptional<SummaryShape>(path.join(dir, summaryFile));
  const files = await listDirOptional(path.join(dir, reportsSubdir));
  const configs: ConfigEvidence[] = [];
  for (const file of files.sort()) {
    if (!file.endsWith('.json')) continue;
    if (file.includes('coinbase')) continue; // corroboration row, not a primary configuration
    const report = await readJsonOptional<{ candidate?: string; verdict?: string; failed_stage?: string }>(
      path.join(dir, reportsSubdir, file)
    );
    if (!report?.candidate) continue;
    const { number, family } = parseCandidate(report.candidate);
    configs.push({
      campaign,
      number,
      family,
      candidate: report.candidate,
      verdict: String(report.verdict ?? 'UNKNOWN'),
      stageReached: report.failed_stage ?? null
    });
  }

  const funnel = emptyFunnel();
  for (const c of configs) {
    const stage = c.stageReached;
    if (!stage) funnel['passed'] += 1;
    else funnel[stage] = (funnel[stage] ?? 0) + 1; // unknown stages keep their own bucket
  }

  const configsEvaluated =
    typeof summary?.configurations_evaluated === 'number' ? summary.configurations_evaluated : configs.length;
  const qualifier = summary?.qualifier ?? null;
  const allFailed = configs.length > 0 && configs.every((c) => c.verdict === 'FAIL');
  // 'FALSIFIED' is a derivation from real data, not a claim: every evaluated config failed, no qualifier.
  const verdict = summary?.verdict ?? (allFailed && qualifier === null ? 'FALSIFIED' : 'UNKNOWN');

  return {
    record: { campaign, configsEvaluated, verdict, qualifier, funnel },
    configs,
    found: summary !== null || configs.length > 0
  };
}

export async function loadEvidence(dir?: string): Promise<EvidencePayload> {
  const evidenceDir = dir ?? resolveEvidenceDir();

  let dirOk = false;
  try {
    const stat = await fs.stat(evidenceDir);
    dirOk = stat.isDirectory();
  } catch {
    dirOk = false;
  }

  const c1 = await loadCampaign(evidenceDir, 1, 'campaign_summary.json', 'reports-c1');
  const c2 = await loadCampaign(evidenceDir, 2, 'campaign2_summary.json', 'reports-c2');

  const available = dirOk && (c1.found || c2.found);
  if (!available) {
    return {
      available: false,
      status: 'UNKNOWN',
      mcV2Status: MC_V2_STATUS,
      funnel: emptyFunnel(),
      campaigns: [],
      configs: []
    };
  }

  const funnel = emptyFunnel();
  for (const rec of [c1.record, c2.record]) {
    for (const s of Object.keys(funnel)) funnel[s] += rec.funnel[s] ?? 0;
  }

  const configs = [...c1.configs, ...c2.configs].sort((a, b) =>
    a.campaign === b.campaign ? a.number - b.number : a.campaign - b.campaign
  );

  return {
    available: true,
    status: 'OK',
    mcV2Status: MC_V2_STATUS,
    funnel,
    campaigns: [c1.record, c2.record],
    configs
  };
}
