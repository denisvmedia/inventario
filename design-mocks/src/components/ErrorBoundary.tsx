import { Component } from "react"
import { TriangleAlert as AlertTriangle, RefreshCcw } from "lucide-react"
import { Button } from "@/components/ui/button"

interface Props {
  children: React.ReactNode
}

interface State {
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error("[ErrorBoundary]", error, info.componentStack)
  }

  private handleReload = () => {
    window.location.reload()
  }

  private handleReset = () => {
    this.setState({ error: null })
  }

  render() {
    const { error } = this.state
    if (!error) return this.props.children

    const isDev = import.meta.env.DEV

    return (
      <div className="min-h-screen bg-background flex items-center justify-center p-6">
        <div className="w-full max-w-lg">
          {/* Icon */}
          <div className="flex justify-center mb-6">
            <div className="flex size-16 items-center justify-center rounded-2xl bg-destructive/10">
              <AlertTriangle className="size-8 text-destructive" />
            </div>
          </div>

          {/* Heading */}
          <div className="text-center mb-8">
            <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight mb-2">
              Something went wrong
            </h1>
            <p className="text-muted-foreground text-sm leading-relaxed max-w-sm mx-auto">
              An unexpected error occurred. Your data is safe — this is a display
              issue. Try reloading the page.
            </p>
          </div>

          {/* Actions */}
          <div className="flex items-center justify-center gap-3 mb-8">
            <Button onClick={this.handleReload} className="gap-2">
              <RefreshCcw className="size-4" />
              Reload page
            </Button>
            <Button variant="outline" onClick={this.handleReset}>
              Try again
            </Button>
          </div>

          {/* Dev-only stack trace */}
          {isDev && (
            <div className="rounded-xl border border-destructive/30 bg-destructive/5 overflow-hidden">
              <div className="px-4 py-2.5 border-b border-destructive/20 flex items-center gap-2">
                <span className="text-xs font-semibold uppercase tracking-widest text-destructive/70">
                  Dev — Error details
                </span>
              </div>
              <div className="p-4 space-y-3 max-h-72 overflow-y-auto">
                <p className="font-mono text-xs font-semibold text-destructive break-all">
                  {error.message}
                </p>
                {error.stack && (
                  <pre className="font-mono text-[11px] text-muted-foreground whitespace-pre-wrap break-all leading-relaxed">
                    {error.stack}
                  </pre>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    )
  }
}
