import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { type DraftCase, TestCaseRows } from "./test-case-editor";

const oneRow: DraftCase[] = [{ stdin: "", expected_output: "" }];

describe("TestCaseRows", () => {
  it("renders one editor per row", () => {
    render(
      <TestCaseRows
        rows={[
          { stdin: "1", expected_output: "1" },
          { stdin: "2", expected_output: "2" },
        ]}
        onChange={() => {}}
      />,
    );
    expect(screen.getAllByTestId("tc-row")).toHaveLength(2);
  });

  it("adds a row", () => {
    const onChange = vi.fn();
    render(<TestCaseRows rows={oneRow} onChange={onChange} />);
    fireEvent.click(screen.getByTestId("add-case"));
    expect(onChange).toHaveBeenCalledWith([
      { stdin: "", expected_output: "" },
      { stdin: "", expected_output: "" },
    ]);
  });

  it("removes a row", () => {
    const onChange = vi.fn();
    render(
      <TestCaseRows
        rows={[
          { stdin: "a", expected_output: "a" },
          { stdin: "b", expected_output: "b" },
        ]}
        onChange={onChange}
      />,
    );
    fireEvent.click(screen.getAllByTestId("tc-remove")[0]!);
    expect(onChange).toHaveBeenCalledWith([{ stdin: "b", expected_output: "b" }]);
  });

  it("edits a field", () => {
    const onChange = vi.fn();
    render(<TestCaseRows rows={oneRow} onChange={onChange} />);
    fireEvent.change(screen.getAllByTestId("tc-stdin")[0]!, { target: { value: "42" } });
    expect(onChange).toHaveBeenCalledWith([{ stdin: "42", expected_output: "" }]);
  });
});
