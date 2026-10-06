/**
 * Client-safe evidence types and funnel constants for the ΩMEGA PRIME Δ
 * operator dashboard. This module has NO Node.js imports so it can be
 * bundled into client components; the filesystem loader lives in
 * src/lib/evidence.ts (server-only).
 */

/**
 * Funnel stages in lab STAGE_ORDER sequence (data_integrity and bootstrap
 * carry no failure counts for completed campaigns, so the funnel tracks the
 * five stages where a fail-fast stop is reported).
 */
export const FUNNEL_STAGES = ['costs', 'walkforward', 'nulls', 'montecarlo', 'overfit'] as const;

export interface ConfigEvidence {
  campaign: number;
  /** Configuration number parsed from the leading digits of the candidate name. */
  number: number;
  /** Strategy family derived from the leading non-parameter segments of the candidate name. */
  family: string;
  candidate: string;
  verdict: string;
  /** Stage the candidate failed at (failed_stage from the lab report), or null if it passed. */
  stageReached: string | null;
}

export interface CampaignEvidence {
  campaign: number;
  configsEvaluated: number;
  /** 'FALSIFIED' is derived only when every evaluated config failed and no qualifier exists. */
  verdict: string;
  qualifier: string | null;
  funnel: Record<string, number>;
}

export interface EvidencePayload {
  available: boolean;
  status: 'OK' | 'UNKNOWN';
  mcV2Status: string;
  funnel: Record<string, number>;
  campaigns: CampaignEvidence[];
  configs: ConfigEvidence[];
}
