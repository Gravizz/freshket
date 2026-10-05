// Item colours for the plate illustration. New items use their colour as the
// item code: the admin picks a hex colour and the code is its six hex digits
// (for example "1A2B3C"), which already fits the code format. The seeded sets
// are named after colours, so they get that colour; any other code gets a
// stable hue derived from it.

const knownColors: Record<string, string> = {
  RED: '#ef4444',
  GREEN: '#22c55e',
  BLUE: '#3b82f6',
  YELLOW: '#f8c21f',
  PINK: '#ec4899',
  PURPLE: '#a855f7',
  ORANGE: '#f97316',
}

const hexCode = /^[0-9A-F]{6}$/

export function itemColor(code: string): string {
  if (hexCode.test(code)) return `#${code}`
  if (knownColors[code]) return knownColors[code]
  let hash = 0
  for (const ch of code) hash = (hash * 31 + ch.charCodeAt(0)) % 360
  return `hsl(${hash} 70% 52%)`
}

// codeFromColor turns a colour input value ("#1a2b3c") into an item code.
export function codeFromColor(color: string): string {
  return color.replace('#', '').toUpperCase()
}

// displayCode shows colour codes the way a designer would write them.
export function displayCode(code: string): string {
  return hexCode.test(code) ? `#${code}` : code
}

// centerColor is Freshket green, the middle of the tray and the first colour offered.
const centerColor = '#00CE7C'

// honeycomb is the admin colour tray: a hexagon of hexagons, three rings
// around Freshket green. Hue follows the angle around the centre and each
// ring out is deeper, so 37 distinct colours fit in a small, familiar shape.
export const honeycomb: string[][] = (() => {
  const radius = 3
  const rows: string[][] = []
  for (let r = -radius; r <= radius; r++) {
    const row: string[] = []
    for (let q = Math.max(-radius, -r - radius); q <= Math.min(radius, -r + radius); q++) {
      const ring = Math.max(Math.abs(q), Math.abs(r), Math.abs(q + r))
      if (ring === 0) {
        row.push(centerColor)
        continue
      }
      const hue = (Math.atan2(r * Math.sqrt(3) / 2, q + r / 2) * 180) / Math.PI
      row.push(hslToHex((hue + 360) % 360, 70, [0, 74, 60, 46][ring]))
    }
    rows.push(row)
  }
  return rows
})()

// firstFreeColor is the first tray colour whose code no item uses yet, or
// undefined when all of them are taken.
export function firstFreeColor(takenCodes: ReadonlySet<string>): string | undefined {
  return [centerColor, ...honeycomb.flat()].find((color) => !takenCodes.has(codeFromColor(color)))
}

function hslToHex(h: number, s: number, l: number): string {
  const a = (s / 100) * Math.min(l / 100, 1 - l / 100)
  const channel = (n: number) => {
    const k = (n + h / 30) % 12
    const v = l / 100 - a * Math.max(-1, Math.min(k - 3, 9 - k, 1))
    return Math.round(v * 255).toString(16).padStart(2, '0')
  }
  return `#${channel(0)}${channel(8)}${channel(4)}`.toUpperCase()
}
