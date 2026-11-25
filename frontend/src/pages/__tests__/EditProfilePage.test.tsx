import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { toast } from "sonner"

import { EditProfilePage } from "@/pages/EditProfilePage"
import { pickRadixSelect } from "@/test/radix"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { stubImageLoading } from "@/test/image"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname} />
}

function renderEdit() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: "/profile/edit",
    routes: (
      <>
        <Route
          path="/profile/edit"
          element={
            <AuthProvider>
              <GroupProvider>
                <ConfirmProvider>
                  <EditProfilePage />
                </ConfirmProvider>
              </GroupProvider>
            </AuthProvider>
          }
        />
        <Route path="*" element={<LocationProbe />} />
      </>
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

const baseUserHandlers = [
  msw.get(api("/auth/me"), () =>
    HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
  ),
  msw.get(api("/groups"), () => HttpResponse.json({ data: [] })),
]

describe("<EditProfilePage />", () => {
  it("validates name is required and ≤255 chars", async () => {
    server.use(...baseUserHandlers)
    const user = userEvent.setup()
    renderEdit()
    // Wait for the form's reset effect to run after the auth probe
    // resolves — otherwise user.clear() races the reset and the field
    // is repopulated with the user's name after we cleared it.
    const nameInput = (await screen.findByTestId("profile-name-input")) as HTMLInputElement
    await waitFor(() => expect(nameInput).toHaveValue("Alex"))
    await user.clear(nameInput)
    await user.click(screen.getByTestId("profile-save"))
    expect(await screen.findByTestId("profile-name-error")).toBeInTheDocument()
  })

  it("submits name + default_group_id and shows the saved-banner inline", async () => {
    let captured: { name: string; default_group_id: string | null } | null = null
    server.use(
      ...baseUserHandlers,
      msw.put(api("/auth/me"), async ({ request }) => {
        captured = (await request.json()) as typeof captured
        return HttpResponse.json({
          id: "u1",
          email: "alex@example.com",
          name: captured?.name,
          default_group_id: captured?.default_group_id ?? undefined,
        })
      })
    )
    const user = userEvent.setup()
    renderEdit()
    const nameInput = (await screen.findByTestId("profile-name-input")) as HTMLInputElement
    await waitFor(() => expect(nameInput).toHaveValue("Alex"))
    await user.clear(nameInput)
    await user.type(nameInput, "Alex 2")
    await user.click(screen.getByTestId("profile-save"))
    // Page no longer redirects to /profile after a save — the user stays put
    // and gets an in-page success-banner with `data-testid="profile-save-success"`.
    await waitFor(() => expect(screen.getByTestId("profile-save-success")).toBeInTheDocument())
    // The form skips sending `default_group_id` when it matches the user's
    // saved value — the seeded user has no default_group_id and the form's
    // select stays empty, so this submit only carries `name`.
    expect(captured).toEqual({ name: "Alex 2" })
  })

  it("default-group is a shadcn Select; picking a group submits default_group_id", async () => {
    // Reference-screen check (#1264): the default-group control is the
    // blessed Radix <Select>, not a native <select>. Drive it via the
    // listbox and confirm the picked id round-trips on submit.
    let captured: { name: string; default_group_id?: string } | null = null
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({
          id: "u1",
          email: "alex@example.com",
          name: "Alex",
          default_group_id: "g2",
        })
      ),
      msw.get(api("/groups"), () =>
        HttpResponse.json({
          data: [
            { id: "g1", type: "groups", attributes: { id: "g1", slug: "one", name: "Group One" } },
            { id: "g2", type: "groups", attributes: { id: "g2", slug: "two", name: "Group Two" } },
          ],
        })
      ),
      msw.put(api("/auth/me"), async ({ request }) => {
        captured = (await request.json()) as typeof captured
        return HttpResponse.json({
          id: "u1",
          email: "alex@example.com",
          name: captured?.name,
          default_group_id: captured?.default_group_id,
        })
      })
    )
    const user = userEvent.setup()
    renderEdit()
    const trigger = await screen.findByTestId("profile-default-group-select")
    // It's the blessed Radix combobox, not a native <select>.
    expect(trigger).toHaveAttribute("role", "combobox")
    await pickRadixSelect(user, /default group/i, { optionLabel: /^Group One$/i })
    await user.click(screen.getByTestId("profile-save"))
    await waitFor(() => expect(screen.getByTestId("profile-save-success")).toBeInTheDocument())
    expect(captured).toEqual({ name: "Alex", default_group_id: "g1" })
  })

  it("password form rejects mismatched confirmation", async () => {
    server.use(...baseUserHandlers)
    const user = userEvent.setup()
    renderEdit()
    await user.click(await screen.findByTestId("password-toggle"))
    await user.type(await screen.findByTestId("current-password"), "old-pw-1")
    await user.type(screen.getByTestId("new-password"), "newer-pw-1")
    await user.type(screen.getByTestId("confirm-password"), "different1")
    await user.click(screen.getByTestId("change-password-submit"))
    expect(await screen.findByTestId("confirm-password-error")).toHaveTextContent(/match/i)
  })

  it("password form rejects when new === current", async () => {
    server.use(...baseUserHandlers)
    const user = userEvent.setup()
    renderEdit()
    await user.click(await screen.findByTestId("password-toggle"))
    await user.type(await screen.findByTestId("current-password"), "samepass1")
    await user.type(screen.getByTestId("new-password"), "samepass1")
    await user.type(screen.getByTestId("confirm-password"), "samepass1")
    await user.click(screen.getByTestId("change-password-submit"))
    expect(await screen.findByTestId("new-password-error")).toHaveTextContent(/differ/i)
  })

  it("posts a successful password change and triggers logout flow", async () => {
    // Scoped fake timers: only this case needs to skip the page's 1500ms
    // post-success delay before `logoutMutation.mutateAsync()` fires.
    // `shouldAdvanceTime: true` keeps msw + react-query happy by letting
    // their microtask/Promise chains run on the host scheduler while the
    // fake clock tracks real time — that's the combination that broke
    // in #1439's earlier attempts (default fake timers swallow the
    // scheduling msw needs to deliver responses).
    vi.useFakeTimers({ shouldAdvanceTime: true })
    try {
      let logoutCalls = 0
      server.use(
        ...baseUserHandlers,
        msw.post(api("/auth/change-password"), () =>
          HttpResponse.json({ message: "Password changed" })
        ),
        msw.post(api("/auth/logout"), () => {
          logoutCalls++
          return new HttpResponse(null, { status: 204 })
        })
      )
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      renderEdit()
      await user.click(await screen.findByTestId("password-toggle"))
      await user.type(await screen.findByTestId("current-password"), "old-pw-1")
      await user.type(screen.getByTestId("new-password"), "new-secure-pw-1")
      await user.type(screen.getByTestId("confirm-password"), "new-secure-pw-1")
      await user.click(screen.getByTestId("change-password-submit"))
      await waitFor(() => expect(screen.getByTestId("password-change-success")).toBeInTheDocument())
      // Fast-forward through the page's 1500ms "let the user read the
      // success state" delay, then await microtasks so the deferred
      // `await logoutMutation.mutateAsync()` makes its msw round-trip.
      await vi.advanceTimersByTimeAsync(1600)
      await waitFor(() => expect(logoutCalls).toBe(1))
    } finally {
      vi.useRealTimers()
    }
  })

  it("surfaces 'incorrect current password' on 422", async () => {
    server.use(
      ...baseUserHandlers,
      msw.post(api("/auth/change-password"), () =>
        HttpResponse.json({ error: "wrong" }, { status: 422 })
      )
    )
    const user = userEvent.setup()
    renderEdit()
    await user.click(await screen.findByTestId("password-toggle"))
    await user.type(await screen.findByTestId("current-password"), "old-pw-1")
    await user.type(screen.getByTestId("new-password"), "new-secure-pw-1")
    await user.type(screen.getByTestId("confirm-password"), "new-secure-pw-1")
    await user.click(screen.getByTestId("change-password-submit"))
    expect(await screen.findByTestId("password-server-error")).toHaveTextContent(
      /current password is incorrect/i
    )
  })
})

describe("<EditProfilePage /> — profile photo (#1382)", () => {
  const photoUrl = "/api/v1/avatars/u1?v=2-abc&uid=u1&exp=1&sig=new"

  beforeEach(() => {
    stubImageLoading()
    // jsdom has no object URLs; the preview only needs a string to show.
    vi.stubGlobal(
      "URL",
      Object.assign(URL, { createObjectURL: () => "blob:preview", revokeObjectURL: () => {} })
    )
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  const photoImage = () =>
    within(screen.getByTestId("profile-photo-avatar")).queryByTestId("user-avatar-image")

  it("uploads a picked photo and shows the stored one", async () => {
    // The body is read as text: request.formData() cannot parse a file part
    // in the jsdom environment. The multipart body itself is complete (see
    // src/test/node-form-data.ts), so the field, file name, type and bytes
    // can all be checked.
    let sentBody = ""
    server.use(
      ...baseUserHandlers,
      msw.post(api("/auth/me/avatar"), async ({ request }) => {
        sentBody = await request.text()
        return HttpResponse.json({ avatar_path: "avatars/u1/2-abc.jpg", avatar_url: photoUrl })
      })
    )
    const user = userEvent.setup()
    renderEdit()
    expect(await screen.findByTestId("profile-photo-upload")).toHaveTextContent("Upload photo")
    expect(screen.queryByTestId("profile-photo-remove")).not.toBeInTheDocument()

    await user.upload(
      screen.getByTestId("profile-photo-input"),
      new File(["jpeg"], "me.jpg", { type: "image/jpeg" })
    )

    await waitFor(() => expect(photoImage()).toHaveAttribute("src", photoUrl))
    expect(sentBody).toContain('name="avatar"; filename="me.jpg"')
    expect(sentBody).toContain("Content-Type: image/jpeg\r\n\r\njpeg\r\n")
    expect(toast.success).toHaveBeenCalledWith("Photo updated.", undefined)
    expect(screen.getByTestId("profile-photo-upload")).toHaveTextContent("Change photo")
    expect(screen.getByTestId("profile-photo-remove")).toBeInTheDocument()
  })

  it("shows the picked file while the upload runs", async () => {
    let finish: () => void = () => {}
    server.use(
      ...baseUserHandlers,
      msw.post(
        api("/auth/me/avatar"),
        () =>
          new Promise<Response>((resolve) => {
            finish = () =>
              resolve(
                HttpResponse.json({ avatar_path: "avatars/u1/2-abc.jpg", avatar_url: photoUrl })
              )
          })
      )
    )
    const user = userEvent.setup()
    renderEdit()
    await user.upload(
      await screen.findByTestId("profile-photo-input"),
      new File(["jpeg"], "me.jpg", { type: "image/jpeg" })
    )

    await waitFor(() => expect(photoImage()).toHaveAttribute("src", "blob:preview"))
    expect(screen.getByTestId("profile-photo-upload")).toHaveTextContent("Uploading…")
    expect(screen.getByTestId("profile-photo-upload")).toBeDisabled()

    finish()
    await waitFor(() => expect(photoImage()).toHaveAttribute("src", photoUrl))
    expect(screen.getByTestId("profile-photo-upload")).toBeEnabled()
  })

  it.each([
    [
      "a GIF",
      new File(["gif"], "me.gif", { type: "image/gif" }),
      "Choose a JPG, PNG or WebP picture.",
    ],
    [
      "a file over 2 MB",
      new File([new Uint8Array(2 * 1024 * 1024 + 1)], "big.png", { type: "image/png" }),
      "That file is larger than 2 MB. Choose a smaller picture.",
    ],
  ])("refuses %s without uploading it", async (_, file, message) => {
    const upload = vi.fn()
    server.use(
      ...baseUserHandlers,
      msw.post(api("/auth/me/avatar"), () => {
        upload()
        return HttpResponse.json({})
      })
    )
    // applyAccept off: the picker's accept list would otherwise hide the GIF,
    // and the check under test is the one behind it.
    const user = userEvent.setup({ applyAccept: false })
    renderEdit()
    await user.upload(await screen.findByTestId("profile-photo-input"), file)

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(message, undefined))
    expect(upload).not.toHaveBeenCalled()
    expect(photoImage()).not.toBeInTheDocument()
  })

  it("says so when the server refuses the image, and keeps the initials", async () => {
    server.use(
      ...baseUserHandlers,
      msw.post(api("/auth/me/avatar"), () =>
        HttpResponse.json(
          { errors: [{ detail: "Animated images are not supported" }] },
          { status: 400 }
        )
      )
    )
    const user = userEvent.setup()
    renderEdit()
    await user.upload(
      await screen.findByTestId("profile-photo-input"),
      new File(["webp"], "moving.webp", { type: "image/webp" })
    )

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        "That image can't be used. Choose a still JPG, PNG or WebP picture.",
        undefined
      )
    )
    await waitFor(() => expect(photoImage()).not.toBeInTheDocument())
    expect(
      within(screen.getByTestId("profile-photo-avatar")).getByTestId("user-avatar-initials")
    ).toHaveTextContent("A")
  })

  it("removes the photo after confirming, back to the initials", async () => {
    const removed = vi.fn()
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({
          id: "u1",
          email: "alex@example.com",
          name: "Alex",
          avatar_path: "avatars/u1/2-abc.jpg",
          avatar_url: photoUrl,
        })
      ),
      msw.get(api("/groups"), () => HttpResponse.json({ data: [] })),
      msw.delete(api("/auth/me/avatar"), () => {
        removed()
        return new HttpResponse(null, { status: 204 })
      })
    )
    const user = userEvent.setup()
    renderEdit()
    await waitFor(() => expect(photoImage()).toHaveAttribute("src", photoUrl))

    await user.click(screen.getByTestId("profile-photo-remove"))
    const dialog = await screen.findByTestId("confirm-dialog")
    expect(dialog).toHaveTextContent("Remove your profile photo?")
    await user.click(within(dialog).getByRole("button", { name: "Remove" }))

    await waitFor(() => expect(photoImage()).not.toBeInTheDocument())
    expect(removed).toHaveBeenCalledTimes(1)
    expect(toast.success).toHaveBeenCalledWith("Photo removed.", undefined)
    expect(screen.queryByTestId("profile-photo-remove")).not.toBeInTheDocument()
  })

  it("leaves the photo alone when the removal is canceled", async () => {
    const removed = vi.fn()
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({
          id: "u1",
          email: "alex@example.com",
          name: "Alex",
          avatar_url: photoUrl,
        })
      ),
      msw.get(api("/groups"), () => HttpResponse.json({ data: [] })),
      msw.delete(api("/auth/me/avatar"), () => {
        removed()
        return new HttpResponse(null, { status: 204 })
      })
    )
    const user = userEvent.setup()
    renderEdit()
    await user.click(await screen.findByTestId("profile-photo-remove"))
    const dialog = await screen.findByTestId("confirm-dialog")
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }))

    await waitFor(() => expect(screen.queryByTestId("confirm-dialog")).not.toBeInTheDocument())
    expect(removed).not.toHaveBeenCalled()
    expect(photoImage()).toHaveAttribute("src", photoUrl)
  })
})
