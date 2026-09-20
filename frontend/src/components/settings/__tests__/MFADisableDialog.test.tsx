import { useState } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { toast } from "sonner"

import { MFADisableDialog } from "@/components/settings/MFADisableDialog"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { apiUrl } from "@/test/handlers"
import { __resetHttpForTests } from "@/lib/http"

interface DisableBody {
  password?: string
  totp_code?: string
  backup_code?: string
}

function disableOk() {
  const seen: { body?: DisableBody } = {}
  server.use(
    msw.post(apiUrl("/auth/mfa/disable"), async ({ request }) => {
      seen.body = (await request.json()) as DisableBody
      return HttpResponse.json({ message: "ok" })
    })
  )
  return seen
}

function Harness({ onOpenChange }: { onOpenChange?: (next: boolean) => void } = {}) {
  const [open, setOpen] = useState(true)
  return (
    <MFADisableDialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        onOpenChange?.(next)
      }}
    />
  )
}

function renderDialog(onOpenChange?: (next: boolean) => void) {
  return renderWithProviders({ children: <Harness onOpenChange={onOpenChange} /> })
}

beforeEach(() => {
  window.localStorage.clear()
  __resetHttpForTests()
  vi.mocked(toast.success).mockClear()
})

describe("<MFADisableDialog />", () => {
  it("requires both a password and a code before it will submit", async () => {
    const user = userEvent.setup()
    renderDialog()

    const confirm = await screen.findByTestId("mfa-disable-confirm")
    expect(confirm).toBeDisabled()

    await user.type(screen.getByTestId("mfa-disable-password"), "hunter2hunter2")
    expect(confirm).toBeDisabled()

    // Whitespace is not a code — re-authentication is the whole point of
    // this dialog.
    await user.type(screen.getByTestId("mfa-disable-code"), "   ")
    expect(confirm).toBeDisabled()

    await user.type(screen.getByTestId("mfa-disable-code"), "123456")
    expect(confirm).toBeEnabled()
  })

  it("sends the authenticator code and closes on success", async () => {
    const user = userEvent.setup()
    const seen = disableOk()
    const onOpenChange = vi.fn()
    renderDialog(onOpenChange)

    await user.type(await screen.findByTestId("mfa-disable-password"), "hunter2hunter2")
    await user.type(screen.getByTestId("mfa-disable-code"), "123456")
    await user.click(screen.getByTestId("mfa-disable-confirm"))

    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(seen.body?.password).toBe("hunter2hunter2")
    expect(seen.body?.totp_code).toBe("123456")
    expect(seen.body?.backup_code).toBeUndefined()
    expect(toast.success).toHaveBeenCalledWith("Two-factor authentication disabled.", undefined)
  })

  it("switches to a backup code and clears what was typed for the other mode", async () => {
    const user = userEvent.setup()
    const seen = disableOk()
    renderDialog()

    await user.type(await screen.findByTestId("mfa-disable-password"), "hunter2hunter2")
    await user.type(screen.getByTestId("mfa-disable-code"), "123456")
    await user.click(screen.getByTestId("mfa-disable-toggle"))

    // A TOTP code left in the box would be posted as a backup code and
    // burn nothing while failing the re-auth.
    const field = screen.getByTestId("mfa-disable-code")
    expect(field).toHaveValue("")
    expect(field).toHaveAttribute("data-mode", "backup")

    await user.type(field, "AAAAA-BBBBB")
    await user.click(screen.getByTestId("mfa-disable-confirm"))

    await waitFor(() => expect(seen.body).toBeDefined())
    expect(seen.body?.backup_code).toBe("AAAAA-BBBBB")
    expect(seen.body?.totp_code).toBeUndefined()
  })

  // 422, not 401: a wrong password here is a re-auth failure, not an
  // expired session, and the client must not answer it with a refresh (#2515).
  it("keeps the dialog open and shows the reason when the server refuses", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    server.use(
      msw.post(apiUrl("/auth/mfa/disable"), () =>
        HttpResponse.json({ error: "password or code is wrong" }, { status: 422 })
      )
    )
    renderDialog(onOpenChange)

    await user.type(await screen.findByTestId("mfa-disable-password"), "wrong-password")
    await user.type(screen.getByTestId("mfa-disable-code"), "123456")
    await user.click(screen.getByTestId("mfa-disable-confirm"))

    expect(await screen.findByTestId("mfa-disable-error")).toHaveTextContent(
      "password or code is wrong"
    )
    expect(onOpenChange).not.toHaveBeenCalled()
    expect(toast.success).not.toHaveBeenCalled()
  })
})
