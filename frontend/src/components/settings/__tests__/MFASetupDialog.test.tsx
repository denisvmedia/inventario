import { useState } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { toast } from "sonner"

import { MFASetupDialog } from "@/components/settings/MFASetupDialog"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { apiUrl } from "@/test/handlers"
import { __resetHttpForTests } from "@/lib/http"

// setupOk registers the enrollment start and returns a counter so a test
// can tell whether re-opening the dialog minted a fresh secret.
function setupOk(secret = "JBSWY3DPEHPK3PXP") {
  const calls = { count: 0 }
  server.use(
    msw.post(apiUrl("/auth/mfa/setup"), () => {
      calls.count += 1
      return HttpResponse.json({
        secret: `${secret}${calls.count}`,
        qr_code_url: `otpauth://totp/Inventario:jane@example.test?secret=${secret}${calls.count}`,
      })
    })
  )
  return calls
}

function setupFails(message = "setup is temporarily unavailable") {
  server.use(
    msw.post(apiUrl("/auth/mfa/setup"), () =>
      HttpResponse.json({ error: message }, { status: 503 })
    )
  )
}

function verifyOk(codes = ["AAAAA-BBBBB", "CCCCC-DDDDD"]) {
  const seen: { code?: string } = {}
  server.use(
    msw.post(apiUrl("/auth/mfa/verify"), async ({ request }) => {
      seen.code = ((await request.json()) as { code?: string }).code
      return HttpResponse.json({ backup_codes: codes })
    })
  )
  return seen
}

// Harness owns the open/close binding the real SettingsPage owns, so the
// tests can close the dialog and re-open it the way a user would.
function Harness({ onOpenChange }: { onOpenChange?: (next: boolean) => void } = {}) {
  const [open, setOpen] = useState(true)
  return (
    <>
      <button type="button" data-testid="reopen" onClick={() => setOpen(true)}>
        open
      </button>
      <MFASetupDialog
        open={open}
        onOpenChange={(next) => {
          setOpen(next)
          onOpenChange?.(next)
        }}
      />
    </>
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

describe("<MFASetupDialog />", () => {
  it("shows the QR code and the manual secret once enrollment starts", async () => {
    setupOk()
    renderDialog()

    expect(await screen.findByTestId("mfa-setup-secret")).toHaveValue("JBSWY3DPEHPK3PXP1")
    expect(screen.getByTestId("mfa-qr")).toBeInTheDocument()
    // Nothing to verify against until the secret lands, and nothing to
    // submit until the user types a code.
    expect(screen.getByTestId("mfa-setup-verify")).toBeDisabled()
  })

  it("reports a failed enrollment start and keeps the form unusable", async () => {
    setupFails()
    renderDialog()

    expect(await screen.findByTestId("mfa-setup-error")).toHaveTextContent(
      "setup is temporarily unavailable"
    )
    expect(screen.getByTestId("mfa-setup-code")).toBeDisabled()
    expect(screen.getByTestId("mfa-setup-verify")).toBeDisabled()
  })

  it("verifies the code and gates Done on acknowledging the backup codes", async () => {
    const user = userEvent.setup()
    setupOk()
    const verify = verifyOk()
    const onOpenChange = vi.fn()
    renderDialog(onOpenChange)

    await screen.findByTestId("mfa-setup-secret")
    await user.type(screen.getByTestId("mfa-setup-code"), " 123456 ")
    await user.click(screen.getByTestId("mfa-setup-verify"))

    const codes = await screen.findByTestId("mfa-backup-codes")
    expect(codes).toHaveTextContent("AAAAA-BBBBB")
    expect(codes).toHaveTextContent("CCCCC-DDDDD")
    expect(verify.code).toBe("123456")

    // The codes are shown once and never again, so Done stays locked until
    // the user says they have them.
    expect(screen.getByTestId("mfa-finish")).toBeDisabled()
    await user.click(screen.getByTestId("mfa-ack-saved"))
    expect(screen.getByTestId("mfa-finish")).toBeEnabled()

    await user.click(screen.getByTestId("mfa-finish"))
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(toast.success).toHaveBeenCalledWith(
      "Two-factor authentication is now active.",
      undefined
    )
  })

  it("keeps the user on the scan step when the code is rejected", async () => {
    const user = userEvent.setup()
    setupOk()
    server.use(
      msw.post(apiUrl("/auth/mfa/verify"), () =>
        HttpResponse.json({ error: "that code is not valid" }, { status: 422 })
      )
    )
    renderDialog()

    await screen.findByTestId("mfa-setup-secret")
    await user.type(screen.getByTestId("mfa-setup-code"), "000000")
    await user.click(screen.getByTestId("mfa-setup-verify"))

    expect(await screen.findByTestId("mfa-setup-error")).toHaveTextContent("that code is not valid")
    // No backup codes were issued — showing them here would tell the user
    // enrollment succeeded when it did not.
    expect(screen.queryByTestId("mfa-backup-codes")).not.toBeInTheDocument()
    expect(screen.getByTestId("mfa-setup-code")).toBeInTheDocument()
  })

  it("copies the backup codes one per line", async () => {
    const user = userEvent.setup()
    setupOk()
    verifyOk()
    renderDialog()

    await screen.findByTestId("mfa-setup-secret")
    await user.type(screen.getByTestId("mfa-setup-code"), "123456")
    await user.click(screen.getByTestId("mfa-setup-verify"))
    await screen.findByTestId("mfa-backup-codes")

    await user.click(screen.getByTestId("mfa-copy-backup"))

    await waitFor(async () =>
      expect(await navigator.clipboard.readText()).toBe("AAAAA-BBBBB\nCCCCC-DDDDD")
    )
  })

  it("mints a fresh secret when the dialog is re-opened", async () => {
    const user = userEvent.setup()
    const calls = setupOk()
    renderDialog()

    expect(await screen.findByTestId("mfa-setup-secret")).toHaveValue("JBSWY3DPEHPK3PXP1")
    await user.click(screen.getByRole("button", { name: "Cancel" }))
    await waitFor(() => expect(screen.queryByTestId("mfa-setup-secret")).not.toBeInTheDocument())

    await user.click(screen.getByTestId("reopen"))

    // A re-open must not replay the abandoned secret: the user may have
    // already half-scanned it into an authenticator.
    expect(await screen.findByTestId("mfa-setup-secret")).toHaveValue("JBSWY3DPEHPK3PXP2")
    expect(calls.count).toBe(2)
  })
})
