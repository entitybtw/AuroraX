import { useState } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { IpListEditor, isLiteralIp } from "./IpListEditor";
import type { ExitStatus } from "@/lib/api/egress-types";

/** Holds the saved list so assertions can read what the editor emitted. */
function Harness({ initial }: { initial: string[] }): JSX.Element {
  const [value, setValue] = useState<string[]>(initial);
  return (
    <div>
      <IpListEditor value={value} onChange={setValue} />
      <output data-testid="value">{value.join(",")}</output>
    </div>
  );
}

/** An exit row the registry would report for a configured source address. */
function exitWith(overrides: Partial<ExitStatus>): ExitStatus {
  return {
    name: "203.0.113.10",
    source: "config",
    kind: "ip",
    tier: 0,
    address: "203.0.113.10",
    available: true,
    disabled: false,
    tier_limited: false,
    failures: 0,
    requests: 4,
    ok: 3,
    ...overrides,
  };
}

/** The same editor wired to live exits and a rotation toggle. */
function LiveHarness({
  initial,
  exits,
  onExitToggle,
}: {
  initial: string[];
  exits: ExitStatus[];
  onExitToggle: (ip: string, disabled: boolean) => void;
}): JSX.Element {
  const [value, setValue] = useState<string[]>(initial);
  return (
    <div>
      <IpListEditor value={value} onChange={setValue} exits={exits} onExitToggle={onExitToggle} />
      <output data-testid="value">{value.join(",")}</output>
    </div>
  );
}

const saved = (): string => screen.getByTestId("value").textContent ?? "";

describe("IpListEditor", () => {
  it("accepts literal addresses only", () => {
    expect(isLiteralIp("203.0.113.10")).toBe(true);
    expect(isLiteralIp("2001:db8::1")).toBe(true);
    expect(isLiteralIp("999.1.1.1")).toBe(false);
    expect(isLiteralIp("203.0.113.10, 10.0.0.1")).toBe(false);
    expect(isLiteralIp("example.com")).toBe(false);
    expect(isLiteralIp("")).toBe(false);
  });

  it("lists every configured address as a checked row", () => {
    render(<Harness initial={["203.0.113.10", "203.0.113.11"]} />);
    expect(screen.getByText("203.0.113.10")).toBeTruthy();
    expect(screen.getByText("203.0.113.11")).toBeTruthy();
    expect(screen.getByText("2 of 2 used")).toBeTruthy();
    expect(saved()).toBe("203.0.113.10,203.0.113.11");
  });

  it("keeps an unchecked address visible but drops it from the saved list", () => {
    render(<Harness initial={["203.0.113.10", "203.0.113.11"]} />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Use 203.0.113.10" }));
    expect(saved()).toBe("203.0.113.11");
    expect(screen.getByText("203.0.113.10")).toBeTruthy();
    expect(screen.getByText("1 of 2 used")).toBeTruthy();
    // The row can be switched back on in the same edit.
    fireEvent.click(screen.getByRole("checkbox", { name: "Not using 203.0.113.10" }));
    expect(saved()).toBe("203.0.113.10,203.0.113.11");
  });

  it("adds an address and refuses invalid or duplicate input", () => {
    render(<Harness initial={["203.0.113.10"]} />);
    const input = screen.getByRole("textbox", { name: "New source IP" });

    fireEvent.change(input, { target: { value: "999.1.1.1" } });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    expect(screen.getByText(/"999.1.1.1" is not a valid IP address/)).toBeTruthy();
    expect(saved()).toBe("203.0.113.10");

    fireEvent.change(input, { target: { value: "203.0.113.10" } });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(screen.getByText("That address is already in the list.")).toBeTruthy();
    expect(saved()).toBe("203.0.113.10");

    fireEvent.change(input, { target: { value: "198.51.100.7" } });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(saved()).toBe("203.0.113.10,198.51.100.7");
    expect(screen.getByText("2 of 2 used")).toBeTruthy();
    expect(screen.queryByText(/is not a valid IP address/)).toBeNull();
  });

  it("removes an address outright", () => {
    render(<Harness initial={["203.0.113.10", "203.0.113.11"]} />);
    fireEvent.click(screen.getByRole("button", { name: "Remove 203.0.113.11" }));
    expect(saved()).toBe("203.0.113.10");
    expect(screen.queryByText("203.0.113.11")).toBeNull();
  });

  it("emits an empty list when every address is unchecked", () => {
    render(<Harness initial={["203.0.113.10"]} />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Use 203.0.113.10" }));
    expect(saved()).toBe("");
    expect(screen.getByText("0 of 1 used")).toBeTruthy();
  });
});

describe("IpListEditor with live exits", () => {
  it("shows the state, the tier and the traffic of each address", () => {
    render(<LiveHarness initial={["203.0.113.10"]} exits={[exitWith({})]} onExitToggle={() => {}} />);
    expect(screen.getByText("in use")).toBeTruthy();
    expect(screen.getByText("rotation")).toBeTruthy();
    expect(screen.getByText("75%")).toBeTruthy();
    expect(screen.getByText("1 registered")).toBeTruthy();
  });

  it("surfaces an address that is cooling down", () => {
    render(
      <LiveHarness
        initial={["203.0.113.10"]}
        exits={[exitWith({ cooldown_until: "2030-01-01T00:00:00Z", failures: 2, available: false })]}
        onExitToggle={() => {}}
      />,
    );
    expect(screen.getByText("cooling down")).toBeTruthy();
    expect(screen.getByText("2 failing")).toBeTruthy();
  });

  it("matches an address the registry normalised", () => {
    render(
      <LiveHarness
        initial={["2001:DB8::1"]}
        exits={[exitWith({ name: "2001:db8::1", address: "2001:db8::1", kind: "ip" })]}
        onExitToggle={() => {}}
      />,
    );
    expect(screen.getByText("in use")).toBeTruthy();
  });

  it("pauses an address in the rotation without changing what is saved", () => {
    const onExitToggle = vi.fn();
    render(<LiveHarness initial={["203.0.113.10"]} exits={[exitWith({})]} onExitToggle={onExitToggle} />);
    fireEvent.click(screen.getByRole("button", { name: "Pause 203.0.113.10 in the rotation" }));
    expect(onExitToggle).toHaveBeenCalledWith("203.0.113.10", true);
    expect(saved()).toBe("203.0.113.10");
  });

  it("resumes an address the operator turned off", () => {
    const onExitToggle = vi.fn();
    render(
      <LiveHarness initial={["203.0.113.10"]} exits={[exitWith({ disabled: true })]} onExitToggle={onExitToggle} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Resume 203.0.113.10 in the rotation" }));
    expect(onExitToggle).toHaveBeenCalledWith("203.0.113.10", false);
  });

  it("expands the full detail of an address", () => {
    render(
      <LiveHarness
        initial={["203.0.113.10"]}
        exits={[exitWith({ weight: 3, last_outcome: "failure", last_error: "timeout talking to upstream" })]}
        onExitToggle={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Show details for 203.0.113.10" }));
    expect(screen.getByText("Source")).toBeTruthy();
    expect(screen.getByText("provider config")).toBeTruthy();
    expect(screen.getByText("Last outcome")).toBeTruthy();
    expect(screen.getByText("failure")).toBeTruthy();
    expect(screen.getByText("Last error")).toBeTruthy();
    expect(screen.getAllByText("timeout talking to upstream").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole("button", { name: "Hide details for 203.0.113.10" }));
    expect(screen.queryByText("Last outcome")).toBeNull();
  });
});
