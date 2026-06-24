// Editor store: selected language and per-problem code drafts, persisted to
// localStorage so an accidental refresh mid-contest never eats code.
// (Drafts are the user's own work, not credentials — localStorage is fine
// here, unlike tokens.)
import { create } from "zustand";
import { persist } from "zustand/middleware";

import type { Language } from "@/lib/schemas";

export const starterCode: Record<Language, string> = {
  cpp: `#include <bits/stdc++.h>
using namespace std;

int main() {
    // your solution here
    return 0;
}
`,
  python: `# your solution here
`,
  go: `package main

import "fmt"

func main() {
	// your solution here
	_ = fmt.Sprint
}
`,
};

function draftKey(problemId: string, language: Language): string {
  return `${problemId}:${language}`;
}

interface EditorState {
  language: Language;
  drafts: Record<string, string>;
  setLanguage: (language: Language) => void;
  getDraft: (problemId: string, language: Language) => string;
  setDraft: (problemId: string, language: Language, code: string) => void;
}

export const useEditorStore = create<EditorState>()(
  persist(
    (set, get) => ({
      language: "python",
      drafts: {},

      setLanguage: (language) => set({ language }),

      getDraft: (problemId, language) =>
        get().drafts[draftKey(problemId, language)] ?? starterCode[language],

      setDraft: (problemId, language, code) =>
        set((state) => ({
          drafts: { ...state.drafts, [draftKey(problemId, language)]: code },
        })),
    }),
    { name: "arena-editor" },
  ),
);
