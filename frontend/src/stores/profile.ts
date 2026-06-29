// Local-only profile preferences. The backend has no profile API yet, so
// these live in localStorage and are clearly a UI-side placeholder. When the
// profile API lands, swap this store's setters for mutations against it — the
// components consuming it won't need to change.
import { create } from "zustand";
import { persist } from "zustand/middleware";

export interface ProfilePrefs {
  displayName: string;
  bio: string;
  website: string;
}

interface ProfileState extends ProfilePrefs {
  setPrefs: (prefs: Partial<ProfilePrefs>) => void;
}

export const useProfileStore = create<ProfileState>()(
  persist(
    (set) => ({
      displayName: "",
      bio: "",
      website: "",
      setPrefs: (prefs) => set(prefs),
    }),
    { name: "arena-profile" },
  ),
);
