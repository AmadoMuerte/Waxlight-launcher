import { create } from "zustand";

interface SupportReportState {
  open: boolean;
  instanceId: string;
  show: (instanceId?: string) => void;
  close: () => void;
}

export const useSupportReportStore = create<SupportReportState>((set) => ({
  open: false,
  instanceId: "",
  show: (instanceId = "") => set({ open: true, instanceId }),
  close: () => set({ open: false }),
}));
