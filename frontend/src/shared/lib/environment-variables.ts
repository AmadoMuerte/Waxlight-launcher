export function formatEnvironmentVariables(values: Record<string, string> | undefined): string {
  return Object.entries(values ?? {})
    .toSorted(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => `${key}=${value}`)
    .join("\n");
}

export function parseEnvironmentVariables(text: string): {
  values: Record<string, string>;
  invalidLine?: number;
} {
  const values: Record<string, string> = {};
  const lines = text.split(/\r?\n/);
  for (let index = 0; index < lines.length; index += 1) {
    const rawLine = lines[index];
    if (rawLine.trim() === "") continue;
    const separator = rawLine.indexOf("=");
    if (separator <= 0) return { values: {}, invalidLine: index + 1 };
    const key = rawLine.slice(0, separator).trim();
    if (!key) return { values: {}, invalidLine: index + 1 };
    values[key] = rawLine.slice(separator + 1);
  }
  return { values };
}

export function environmentVariablesEqual(
  left: Record<string, string> | undefined,
  right: Record<string, string> | undefined,
): boolean {
  return formatEnvironmentVariables(left) === formatEnvironmentVariables(right);
}
