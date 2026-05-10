import { Component, type ErrorInfo, type ReactNode } from "react"

import { UnexpectedErrorPage } from "@/pages/UnexpectedErrorPage"

interface RootErrorBoundaryProps {
  children: ReactNode
}

interface RootErrorBoundaryState {
  error: Error | null
  errorInfo: ErrorInfo | null
}

// RootErrorBoundary turns any uncaught render-time exception into a
// proper error page (`UnexpectedErrorPage`) instead of a blank white
// screen. Sits below `<Providers>` so the page can use i18n / theme,
// but above `<AppRoutes>` so it catches every route's rendering
// failure. Outside this boundary's reach: errors that throw in event
// handlers, async work after first render, and anything in the
// providers themselves — all standard React error-boundary limits.
export class RootErrorBoundary extends Component<
  RootErrorBoundaryProps,
  RootErrorBoundaryState
> {
  state: RootErrorBoundaryState = { error: null, errorInfo: null }

  static getDerivedStateFromError(error: Error): Partial<RootErrorBoundaryState> {
    return { error }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    this.setState({ errorInfo })
    // Surface to the console so dev tools / error-tracking integrations
    // still capture the original throw site.
    console.error("[RootErrorBoundary]", error, errorInfo)
  }

  reset = (): void => {
    this.setState({ error: null, errorInfo: null })
  }

  render(): ReactNode {
    if (this.state.error) {
      return (
        <UnexpectedErrorPage
          error={this.state.error}
          errorInfo={this.state.errorInfo}
          onReset={this.reset}
        />
      )
    }
    return this.props.children
  }
}
