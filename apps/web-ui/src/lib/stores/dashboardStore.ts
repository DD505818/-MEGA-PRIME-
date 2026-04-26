import { create } from 'zustand';

export const useDashboardStore = create<{ selectedSymbol: string; setSelectedSymbol: (s: string) => void }>((set) => ({
  selectedSymbol: 'BTC-USD',
  setSelectedSymbol: (selectedSymbol) => set({ selectedSymbol })
}));
