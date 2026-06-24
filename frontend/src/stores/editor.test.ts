import { beforeEach, describe, expect, it } from "vitest";

import { starterCode, useEditorStore } from "./editor";

describe("editor store", () => {
  beforeEach(() => {
    useEditorStore.setState({ language: "python", drafts: {} });
  });

  it("returns starter code for untouched problems", () => {
    const { getDraft } = useEditorStore.getState();
    expect(getDraft("p1", "python")).toBe(starterCode.python);
    expect(getDraft("p1", "cpp")).toBe(starterCode.cpp);
  });

  it("isolates drafts per problem and per language", () => {
    const { setDraft, getDraft } = useEditorStore.getState();

    setDraft("p1", "python", "print(1)");
    setDraft("p1", "cpp", "int main(){}");
    setDraft("p2", "python", "print(2)");

    expect(getDraft("p1", "python")).toBe("print(1)");
    expect(getDraft("p1", "cpp")).toBe("int main(){}");
    expect(getDraft("p2", "python")).toBe("print(2)");
    expect(getDraft("p2", "cpp")).toBe(starterCode.cpp);
  });

  it("persists drafts and language to localStorage", () => {
    useEditorStore.getState().setDraft("p1", "go", "package main");
    useEditorStore.getState().setLanguage("go");

    const raw = localStorage.getItem("arena-editor");
    expect(raw).not.toBeNull();
    const stored = JSON.parse(raw ?? "{}") as {
      state: { drafts: Record<string, string>; language: string };
    };
    expect(stored.state.drafts["p1:go"]).toBe("package main");
    expect(stored.state.language).toBe("go");
  });
});
