// @vitest-environment jsdom

import { fireEvent } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";

import { installInputModality } from "./input-modality";

let cleanup = () => {};

afterEach(() => cleanup());

it("tracks pointer and keyboard input", () => {
  cleanup = installInputModality();
  expect(document.documentElement.dataset.inputModality).toBe("keyboard");

  fireEvent.pointerDown(document);
  expect(document.documentElement.dataset.inputModality).toBe("pointer");

  fireEvent.keyDown(document, { key: "Enter", altKey: true, ctrlKey: true });
  expect(document.documentElement.dataset.inputModality).toBe("keyboard");
});
