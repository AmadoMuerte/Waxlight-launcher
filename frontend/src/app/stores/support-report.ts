import { create } from "zustand";

interface SupportReportState {
  open: boolean;
  show: () => void;
  close: () => void;
}

export const useSupportReportStore = create<SupportReportState>((set) => ({
  open: false,
  show: () => set({ open: true }),
  close: () => set({ open: false }),
}));
