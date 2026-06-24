// Auth store: who is signed in and whether the session has been restored.
// The access token itself lives in lib/api.ts module memory; this store
// mirrors only the user identity and session status for the UI.
import { create } from "zustand";

import { apiFetch, registerSessionExpiredHandler, refreshSession, setAccessToken } from "@/lib/api";
import { userSchema, type AuthResponse, type User } from "@/lib/schemas";

export type AuthStatus = "restoring" | "authenticated" | "guest";

interface AuthState {
  status: AuthStatus;
  user: User | null;
  // bootstrap restores the session from the refresh cookie on first load.
  bootstrap: () => Promise<void>;
  // signedIn applies a successful login/signup response.
  signedIn: (resp: AuthResponse) => void;
  signOut: () => Promise<void>;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  status: "restoring",
  user: null,

  bootstrap: async () => {
    registerSessionExpiredHandler(() => {
      setAccessToken(null);
      set({ status: "guest", user: null });
    });

    const ok = await refreshSession();
    if (!ok) {
      set({ status: "guest", user: null });
      return;
    }
    try {
      const user = await apiFetch("/api/v1/me", { schema: userSchema, auth: true });
      set({ status: "authenticated", user });
    } catch {
      set({ status: "guest", user: null });
    }
  },

  signedIn: (resp) => {
    setAccessToken(resp.access_token);
    set({ status: "authenticated", user: resp.user });
  },

  signOut: async () => {
    try {
      await apiFetch("/api/v1/auth/logout", { method: "POST" });
    } catch {
      // Logout is best-effort; the cookie may already be gone.
    }
    setAccessToken(null);
    if (get().status !== "guest") {
      set({ status: "guest", user: null });
    }
  },
}));
