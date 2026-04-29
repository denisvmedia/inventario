import { Button } from "@/components/ui/button"

export function HomePage() {
  return (
    <section
      aria-labelledby="home-title"
      className="flex flex-col gap-4 max-w-md w-full text-center"
    >
      <h1 id="home-title" className="scroll-m-20 text-3xl font-semibold tracking-tight">
        Inventario
      </h1>
      <p className="text-muted-foreground text-sm">
        React frontend scaffold — features land in subsequent issues under epic #1397.
      </p>
      <div className="flex justify-center pt-2">
        <Button variant="default" size="default">
          Get started
        </Button>
      </div>
    </section>
  )
}
