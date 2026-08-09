import { create } from "zustand";

interface ModSelectionState {
  selectedModIds: string[];
  setSelected: (modId: string, selected: boolean) => void;
  clear: () => void;
}

export const useModSelectionStore = create<ModSelectionState>((set) => ({
  selectedModIds: [],
  setSelected: (modId, selected) =>
    set((state) => ({
      selectedModIds: selected
        ? state.selectedModIds.includes(modId)
          ? state.selectedModIds
          : [...state.selectedModIds, modId]
        : state.selectedModIds.filter((id) => id !== modId),
    })),
  clear: () => set({ selectedModIds: [] }),
}));
