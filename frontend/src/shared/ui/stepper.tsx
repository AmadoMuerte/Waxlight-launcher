interface StepperProps {
  value: number;
  min: number;
  max: number;
  onChange: (value: number) => void;
  label?: string;
  decreaseLabel?: string;
  increaseLabel?: string;
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

export function Stepper({
  value,
  min,
  max,
  onChange,
  label,
  decreaseLabel = "Decrease",
  increaseLabel = "Increase",
}: StepperProps) {
  return (
    <div className="stepper">
      <button
        type="button"
        className="stepperButton"
        aria-label={decreaseLabel}
        onClick={() => onChange(clamp(value - 1, min, max))}
        disabled={value <= min}
      >
        <svg
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
        >
          <path d="M5 12h14" />
        </svg>
      </button>
      <input
        type="number"
        className="stepperInput"
        aria-label={label}
        min={min}
        max={max}
        value={value}
        onChange={(event) => {
          const next = Number(event.target.value);
          if (Number.isInteger(next)) {
            onChange(clamp(next, min, max));
          }
        }}
      />
      <button
        type="button"
        className="stepperButton"
        aria-label={increaseLabel}
        onClick={() => onChange(clamp(value + 1, min, max))}
        disabled={value >= max}
      >
        <svg
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
        >
          <path d="M12 5v14M5 12h14" />
        </svg>
      </button>
    </div>
  );
}
