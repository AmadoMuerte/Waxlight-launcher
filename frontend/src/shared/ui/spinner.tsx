export function Spinner({ className = "" }: { className?: string }) {
  return <span className={`spinner ${className}`.trim()} />;
}
