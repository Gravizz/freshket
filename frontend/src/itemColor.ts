// Item colours for the plate illustration. The seeded sets are named
// after colours, so they get that colour; items an admin adds later get a
// stable hue derived from their code.

const knownColors: Record<string, string> = {
  RED: '#ef4444',
  GREEN: '#22c55e',
  BLUE: '#3b82f6',
  YELLOW: '#f8c21f',
  PINK: '#ec4899',
  PURPLE: '#a855f7',
  ORANGE: '#f97316',
}

export function itemColor(code: string): string {
  if (knownColors[code]) return knownColors[code]
  let hash = 0
  for (const ch of code) hash = (hash * 31 + ch.charCodeAt(0)) % 360
  return `hsl(${hash} 70% 52%)`
}
