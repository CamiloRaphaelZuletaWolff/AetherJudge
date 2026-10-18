import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it } from "vitest";
import { Tabs } from "./tabs";

function Example() {
  const [value, setValue] = useState("first");
  return (
    <Tabs
      value={value}
      onChange={setValue}
      options={[
        { value: "first", label: "First" },
        { value: "second", label: "Second" },
        { value: "third", label: "Third" },
      ]}
    />
  );
}
describe("tab keyboard navigation", () => {
  it("uses one tab stop and supports arrows, wrapping, Home, and End", () => {
    render(<Example />);
    const first = screen.getByRole("tab", { name: "First" });
    const second = screen.getByRole("tab", { name: "Second" });
    const third = screen.getByRole("tab", { name: "Third" });
    first.focus();
    expect(second).toHaveAttribute("tabindex", "-1");
    fireEvent.keyDown(first, { key: "ArrowRight" });
    expect(second).toHaveFocus();
    expect(second).toHaveAttribute("aria-selected", "true");
    fireEvent.keyDown(second, { key: "End" });
    expect(third).toHaveFocus();
    fireEvent.keyDown(third, { key: "ArrowRight" });
    expect(first).toHaveFocus();
    fireEvent.keyDown(first, { key: "ArrowLeft" });
    expect(third).toHaveFocus();
    fireEvent.keyDown(third, { key: "Home" });
    expect(first).toHaveFocus();
  });
});
