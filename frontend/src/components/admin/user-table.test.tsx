import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { AdminUser } from "@/lib/schemas";

import { UserRow } from "./user-table";

function user(overrides: Partial<AdminUser> = {}): AdminUser {
  return {
    id: "11111111-1111-4111-8111-111111111111",
    username: "bob",
    email: "bob@example.com",
    role: "user",
    created_at: new Date().toISOString(),
    ...overrides,
  };
}

describe("UserRow", () => {
  it("shows the user's role label", () => {
    render(
      <UserRow
        user={user({ role: "moderator" })}
        isSelf={false}
        pending={false}
        onRoleChange={() => {}}
      />,
    );
    // Badge + the selected <option> both carry the label.
    expect(screen.getAllByText("Moderator").length).toBeGreaterThan(0);
  });

  it("fires onRoleChange with the chosen role", () => {
    const onRoleChange = vi.fn();
    render(<UserRow user={user()} isSelf={false} pending={false} onRoleChange={onRoleChange} />);
    fireEvent.change(screen.getByTestId("role-select"), { target: { value: "admin" } });
    expect(onRoleChange).toHaveBeenCalledWith("admin");
  });

  it("locks the current admin's own row (anti-lockout)", () => {
    render(
      <UserRow user={user({ role: "admin" })} isSelf pending={false} onRoleChange={() => {}} />,
    );
    expect(screen.getByTestId("role-select")).toBeDisabled();
    expect(screen.getByText("(you)")).toBeInTheDocument();
  });

  it("disables the select while a change is pending", () => {
    render(<UserRow user={user()} isSelf={false} pending onRoleChange={() => {}} />);
    expect(screen.getByTestId("role-select")).toBeDisabled();
  });
});
