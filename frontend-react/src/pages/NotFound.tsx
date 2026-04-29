import { Link } from "react-router-dom"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { RouteTitle } from "@/components/routing/RouteTitle"

export function NotFoundPage() {
  const { t } = useTranslation()
  return (
    <>
      <RouteTitle title={t("errors:notFound.documentTitle")} />
      <section
        aria-labelledby="notfound-title"
        data-testid="page-not-found"
        className="flex flex-col gap-4 max-w-md w-full text-center"
      >
        <h1 id="notfound-title" className="scroll-m-20 text-3xl font-semibold tracking-tight">
          {t("errors:notFound.heading")}
        </h1>
        <p className="text-muted-foreground text-sm">{t("errors:notFound.description")}</p>
        <div className="flex justify-center pt-2">
          <Button asChild variant="default" size="default">
            <Link to="/">{t("common:actions.goHome")}</Link>
          </Button>
        </div>
      </section>
    </>
  )
}
