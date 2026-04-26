import { Role } from '@/types';

export const canTrade = (role: Role) => role === 'admin' || role === 'trader' || role === 'operator';
export const canKillSwitch = (role: Role) => role === 'admin' || role === 'operator';
