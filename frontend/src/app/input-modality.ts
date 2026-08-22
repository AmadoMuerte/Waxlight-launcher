export function installInputModality(documentTarget: Document = document) {
  const setPointer = () => {
    documentTarget.documentElement.dataset.inputModality = "pointer";
  };
  const setKeyboard = (event: KeyboardEvent) => {
    if (!["Alt", "Control", "Meta", "Shift"].includes(event.key)) {
      documentTarget.documentElement.dataset.inputModality = "keyboard";
    }
  };

  documentTarget.documentElement.dataset.inputModality = "keyboard";
  documentTarget.addEventListener("pointerdown", setPointer, true);
  documentTarget.addEventListener("keydown", setKeyboard, true);

  return () => {
    documentTarget.removeEventListener("pointerdown", setPointer, true);
    documentTarget.removeEventListener("keydown", setKeyboard, true);
    delete documentTarget.documentElement.dataset.inputModality;
  };
}
