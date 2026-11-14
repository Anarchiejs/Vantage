/** Vantage mark: a sight on a target, drawn so it renders the same with every font. */
export function BrandMark({ size = 32 }: { size?: number }) {
  return (
    <svg className="brand-mark" width={size} height={size} viewBox="0 0 32 32" aria-hidden="true">
      <rect width="32" height="32" rx="9" fill="#147d60" />
      <circle cx="16" cy="16" r="8.5" fill="none" stroke="#fff" strokeWidth="2" />
      <circle cx="16" cy="16" r="3.5" fill="#fff" />
      <path d="M16 4v4M16 24v4M4 16h4M24 16h4" stroke="#fff" strokeWidth="2" strokeLinecap="round" opacity=".55" />
    </svg>
  )
}
