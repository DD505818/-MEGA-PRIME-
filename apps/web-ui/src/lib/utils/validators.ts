import { z } from 'zod';

export const riskLimitsSchema = z.object({
  dailyLossLimit: z.number().positive(),
  maxDrawdown: z.number().min(0).max(100),
  riskPerTrade: z.number().min(0).max(100)
});
