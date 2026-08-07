// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";

import { Field } from "@/shared/ui/field";

import { Stepper } from "./stepper";

afterEach(() => cleanup());

it("renders the current value and enables the buttons within the range", () => {
  render(
    <Field label="Parallel downloads">
      <Stepper label="Parallel downloads" value={3} min={1} max={10} onChange={vi.fn()} />
    </Field>,
  );

  const input = screen.getByRole("spinbutton", { name: "Parallel downloads" }) as HTMLInputElement;
  expect(input.value).toBe("3");
  const decrease = screen.getByRole("button", { name: "Decrease" }) as HTMLButtonElement;
  const increase = screen.getByRole("button", { name: "Increase" }) as HTMLButtonElement;
  expect(decrease.disabled).toBe(false);
  expect(increase.disabled).toBe(false);
});

it("disables the minus button at the minimum", () => {
  render(<Stepper value={1} min={1} max={10} onChange={vi.fn()} />);
  const decrease = screen.getByRole("button", { name: "Decrease" }) as HTMLButtonElement;
  const increase = screen.getByRole("button", { name: "Increase" }) as HTMLButtonElement;
  expect(decrease.disabled).toBe(true);
  expect(increase.disabled).toBe(false);
});

it("disables the plus button at the maximum", () => {
  render(<Stepper value={10} min={1} max={10} onChange={vi.fn()} />);
  const increase = screen.getByRole("button", { name: "Increase" }) as HTMLButtonElement;
  expect(increase.disabled).toBe(true);
});

it("increments and decrements through the buttons", async () => {
  const onChange = vi.fn();
  render(<Stepper value={3} min={1} max={10} onChange={onChange} />);

  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Increase" }));
  await user.click(screen.getByRole("button", { name: "Decrease" }));

  expect(onChange).toHaveBeenNthCalledWith(1, 4);
  expect(onChange).toHaveBeenNthCalledWith(2, 2);
});

it("clamps typed values to the allowed range", () => {
  const onChange = vi.fn();
  render(<Stepper value={3} min={1} max={10} onChange={onChange} />);

  const input = screen.getByRole("spinbutton");
  fireEvent.change(input, { target: { value: "12" } });
  fireEvent.change(input, { target: { value: "0" } });

  expect(onChange).toHaveBeenNthCalledWith(1, 10);
  expect(onChange).toHaveBeenNthCalledWith(2, 1);
});

it("uses the provided labels for the buttons", () => {
  render(
    <Stepper
      value={5}
      min={1}
      max={10}
      onChange={vi.fn()}
      decreaseLabel="Уменьшить"
      increaseLabel="Увеличить"
    />,
  );

  expect(screen.getByRole("button", { name: "Уменьшить" })).toBeTruthy();
  expect(screen.getByRole("button", { name: "Увеличить" })).toBeTruthy();
});
