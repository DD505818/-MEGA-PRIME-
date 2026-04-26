import { create } from 'zustand';
import type { Mode } from '@/types';

export const useSystemStore = create<{ mode: Mode; setMode: (m: Mode) => void }>((set) => ({
  mode: 'paper',
  setMode: (mode) => set({ mode })
}));
