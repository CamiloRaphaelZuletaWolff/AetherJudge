"use client";

// Minimal theme system (no external dependency). The initial class is applied
// by the no-flash script in layout.tsx; this provider keeps React state in
// sync, persists the choice, and follows the OS until the user picks a side.
import { createContext, useCallback, useContext, useEffect, useState } from "react";

export type Theme = "light" | "dark";

interface ThemeContextValue {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  toggle: () => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);
const STORAGE_KEY = "arena-theme";

function apply(theme: Theme) {
  const el = document.documentElement;
  el.classList.toggle("dark", theme === "dark");
  el.style.colorScheme = theme;
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  // Lazy-init from the class the no-flash script already resolved (no effect →
  // no cascading render). Falls back to "dark" during SSR where there's no DOM.
  const [theme, setThemeState] = useState<Theme>(() =>
    typeof document !== "undefined" && document.documentElement.classList.contains("dark")
      ? "dark"
      : "light",
  );

  // Follow the OS only while the user hasn't made an explicit choice.
  useEffect(() => {
    if (typeof window === "undefined" || !window.matchMedia) return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => {
      if (localStorage.getItem(STORAGE_KEY)) return;
      const next: Theme = mq.matches ? "dark" : "light";
      apply(next);
      setThemeState(next);
    };
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  const setTheme = useCallback((next: Theme) => {
    localStorage.setItem(STORAGE_KEY, next);
    apply(next);
    setThemeState(next);
  }, []);

  const toggle = useCallback(() => {
    setTheme(document.documentElement.classList.contains("dark") ? "light" : "dark");
  }, [setTheme]);

  return (
    <ThemeContext.Provider value={{ theme, setTheme, toggle }}>{children}</ThemeContext.Provider>
  );
}

// Tolerant default outside a provider: lets isolated component tests render
// theme-aware UI without wiring the whole provider tree. In the app the real
// provider always wraps the tree (see providers.tsx).
const fallback: ThemeContextValue = {
  theme: "dark",
  setTheme: () => {},
  toggle: () => {},
};

export function useTheme(): ThemeContextValue {
  return useContext(ThemeContext) ?? fallback;
}
