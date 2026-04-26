import { create } from 'zustand';

export const useUiStore = create<{ sidebarOpen: boolean; setSidebarOpen: (v: boolean) => void }>((set) => ({
  sidebarOpen: true,
  setSidebarOpen: (sidebarOpen) => set({ sidebarOpen })
}));
