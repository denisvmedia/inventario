import { Fragment } from "react"

import type { ChangelogInline } from "@/lib/changelog"

// ChangelogText renders the inline tokens of a changelog entry. The tokens
// come from the build-time parser, so there is no Markdown or HTML to
// interpret here; links are http(s) by construction and open in a new tab.
export function ChangelogText({ inlines }: { inlines: ChangelogInline[] }) {
  return (
    <>
      {inlines.map((node, i) => {
        switch (node.type) {
          case "text":
            return <Fragment key={i}>{node.text}</Fragment>
          case "code":
            return (
              <code
                key={i}
                className="rounded bg-muted px-[0.3rem] py-[0.1rem] font-mono text-[0.85em] break-words"
              >
                {node.text}
              </code>
            )
          case "strong":
            return (
              <strong key={i} className="font-semibold">
                <ChangelogText inlines={node.children} />
              </strong>
            )
          case "link":
            return (
              <a
                key={i}
                href={node.href}
                target="_blank"
                rel="noopener noreferrer"
                className="font-medium text-primary underline-offset-4 hover:underline"
              >
                <ChangelogText inlines={node.children} />
              </a>
            )
        }
      })}
    </>
  )
}
