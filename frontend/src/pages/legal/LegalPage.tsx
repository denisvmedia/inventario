import type { ReactNode } from "react"
import { Link } from "react-router-dom"
import { ArrowLeft } from "lucide-react"

import { RouteTitle } from "@/components/routing/RouteTitle"

interface LegalPageProps {
  title: string
  updated: string
  children: ReactNode
}

// Shared shell for /privacy and /terms. Public: a visitor has to be able to
// read what they are agreeing to before the account exists that would let
// them in.
export function LegalPage({ title, updated, children }: LegalPageProps) {
  return (
    <div className="min-h-svh bg-background">
      <RouteTitle title={title} />
      <div className="mx-auto w-full max-w-3xl px-6 py-12">
        <Link
          to="/register"
          className="mb-8 inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="size-4" aria-hidden="true" />
          Back
        </Link>
        <h1 className="text-3xl font-semibold tracking-tight">{title}</h1>
        <p className="mt-2 text-sm text-muted-foreground">Last updated: {updated}</p>
        <div className="prose-legal mt-8 space-y-6 text-sm leading-relaxed">{children}</div>
      </div>
    </div>
  )
}

export function LegalSection({ heading, children }: { heading: string; children: ReactNode }) {
  return (
    <section className="space-y-2">
      <h2 className="text-base font-semibold text-foreground">{heading}</h2>
      <div className="space-y-2 text-muted-foreground">{children}</div>
    </section>
  )
}
