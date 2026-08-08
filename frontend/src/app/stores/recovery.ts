import { create } from "zustand";

import type { RecoverySuggestion } from "../../entities/last-known-good/model";

// RecoveryStore holds the pending recovery suggestion published by the backend
// after a failed launch. The backend never rolls back automatically; the
// dialog rendered from this store always asks the user first.
//
// dismissedSignatures is the lightweight acknowledgement mechanism: when the
// user keeps the current (failed) state, its configuration signature is
// recorded in memory and repeat prompts for the exact same state are skipped
// during this session. A materially changed configuration produces a new
// signature and can show the prompt again.
interface RecoveryState {
  suggestion?: RecoverySuggestion;
  restoring: boolean;
  dismissedSignatures: Record<string, string>;
  show: (suggestion: RecoverySuggestion) => void;
  acknowledge: () => void;
  hide: () => void;
  setRestoring: (restoring: boolean) => void;
  isDismissed: (suggestion: RecoverySuggestion) => boolean;
}

export const useRecoveryStore = create<RecoveryState>((set, get) => ({
  restoring: false,
  dismissedSignatures: {},
  show: (suggestion) => {
    if (get().isDismissed(suggestion)) {
      return;
    }
    set({ suggestion });
  },
  acknowledge: () => {
    const { suggestion, restoring } = get();
    if (!suggestion || restoring) {
      return;
    }
    set((state) => ({
      suggestion: undefined,
      dismissedSignatures: {
        ...state.dismissedSignatures,
        [suggestion.instanceId]: suggestion.stateSignature,
      },
    }));
  },
  hide: () => set({ suggestion: undefined, restoring: false }),
  setRestoring: (restoring) => set({ restoring }),
  isDismissed: (suggestion) =>
    get().dismissedSignatures[suggestion.instanceId] === suggestion.stateSignature,
}));
