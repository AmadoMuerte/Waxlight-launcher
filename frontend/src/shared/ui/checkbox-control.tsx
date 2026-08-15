import type { InputHTMLAttributes } from "react";

import { Checkbox as RadixCheckbox } from "./checkbox";

export function Checkbox({
  label,
  className = "",
  onChange,
  checked,
  title,
}: Omit<InputHTMLAttributes<HTMLInputElement>, "type"> & {
  label: string;
}) {
  return (
    <label className={`checkboxControl ${className}`.trim()}>
      <RadixCheckbox
        checked={checked}
        onCheckedChange={(radixChecked) => {
          // Adapt Radix boolean callback to the native checkbox onChange API expected by consumers.
          // oxlint-disable-next-line typescript/no-unsafe-type-assertion -- intentional adapter between Radix and native checkbox APIs
          onChange?.({
            target: { checked: radixChecked === true },
          } as React.ChangeEvent<HTMLInputElement>);
        }}
      />
      <span title={title}>{label}</span>
    </label>
  );
}
