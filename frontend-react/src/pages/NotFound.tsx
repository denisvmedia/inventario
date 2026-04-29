import { Link } from "react-router-dom"

import { Button } from "@/components/ui/button"
import { RouteTitle } from "@/components/routing/RouteTitle"

export function NotFoundPage() {
  return (
    <>
      <RouteTitle title="Page not found" />
      <section
        aria-labelledby="notfound-title"
        data-testid="page-not-found"
        className="flex flex-col gap-4 max-w-md w-full text-center"
      >
        <h1 id="notfound-title" className="scroll-m-20 text-3xl font-semibold tracking-tight">
          Page not found
        </h1>
        <p className="text-muted-foreground text-sm">
          The page you requested doesn’t exist or has been moved.
        </p>
        <div className="flex justify-center pt-2">
          <Button asChild variant="default" size="default">
            <Link to="/">Go home</Link>
          </Button>
        </div>
      </section>
    </>
  )
}
