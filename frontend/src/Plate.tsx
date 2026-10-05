import { itemColor } from './itemColor'

// A plate illustration tinted by the item's colour (see itemColor).
type Props = { code: string; size?: 'sm' | 'lg'; className?: string }

export default function Plate({ code, size = 'lg', className = '' }: Props) {
  const color = itemColor(code)
  const box = size === 'lg' ? 'size-20' : 'size-9'
  return (
    <span
      aria-hidden
      className={`relative inline-grid shrink-0 place-items-center rounded-full bg-white shadow-[inset_0_-3px_0_rgb(0_0_0/0.06),0_6px_14px_-6px_rgb(0_0_0/0.25)] ring-1 ring-black/5 ${box} ${className}`}
    >
      <span
        className="size-[68%] rounded-full"
        style={{
          background: `radial-gradient(circle at 32% 28%, color-mix(in oklab, ${color}, white 45%), ${color} 55%, color-mix(in oklab, ${color}, black 18%))`,
          boxShadow: `0 0 0 3px color-mix(in oklab, ${color}, white 80%)`,
        }}
      />
      <span className="absolute top-[22%] left-[30%] h-[12%] w-[18%] -rotate-30 rounded-full bg-white/70 blur-[1px]" />
    </span>
  )
}
