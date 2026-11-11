import type { ReactNode } from 'react'
export function Header({ title, description, action }: { title: string; description: string; action?: ReactNode }) {
  return <header className="page-heading"><div><p className="eyebrow">YOUR DEVELOPMENT SPACE</p><h1>{title}</h1><p className="page-description">{description}</p></div>{action}</header>
}
