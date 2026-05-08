import { useState } from "react"
import { ArrowLeft, Camera, Eye, EyeOff, CircleCheck as CheckCircle2, TriangleAlert as AlertTriangle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import { cn } from "@/lib/utils"

interface EditProfileViewProps {
  onBack: () => void
}

function Section({ title, description, children }: { title: string; description?: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:gap-8">
      <div className="sm:w-56 shrink-0">
        <p className="text-sm font-semibold">{title}</p>
        {description && <p className="text-xs text-muted-foreground mt-0.5 leading-relaxed">{description}</p>}
      </div>
      <div className="flex-1 flex flex-col gap-4">{children}</div>
    </div>
  )
}

function PasswordInput({
  id, label, value, onChange, placeholder, hint,
}: {
  id: string; label: string; value: string; onChange: (v: string) => void; placeholder?: string; hint?: string
}) {
  const [show, setShow] = useState(false)
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <div className="relative">
        <Input
          id={id}
          type={show ? "text" : "password"}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          className="pr-10"
        />
        <button
          type="button"
          onClick={() => setShow((v) => !v)}
          className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
        >
          {show ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
        </button>
      </div>
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  )
}

function strengthScore(pw: string): number {
  if (!pw) return 0
  let score = 0
  if (pw.length >= 8) score++
  if (pw.length >= 12) score++
  if (/[A-Z]/.test(pw)) score++
  if (/[0-9]/.test(pw)) score++
  if (/[^A-Za-z0-9]/.test(pw)) score++
  return score
}

function PasswordStrength({ password }: { password: string }) {
  if (!password) return null
  const score = strengthScore(password)
  const label = ["Very weak", "Weak", "Fair", "Good", "Strong"][Math.min(score - 1, 4)] ?? "Very weak"
  const color = score <= 1 ? "bg-destructive" : score === 2 ? "bg-status-expiring" : score === 3 ? "bg-chart-4" : "bg-status-active"
  return (
    <div className="flex flex-col gap-1.5 mt-1">
      <div className="flex gap-1">
        {[1, 2, 3, 4, 5].map((i) => (
          <div
            key={i}
            className={cn(
              "h-1 flex-1 rounded-full transition-colors",
              i <= score ? color : "bg-border"
            )}
          />
        ))}
      </div>
      <p className="text-xs text-muted-foreground">{label}</p>
    </div>
  )
}

export function EditProfileView({ onBack }: EditProfileViewProps) {
  // Profile fields
  const [displayName, setDisplayName] = useState("Alex Johnson")
  const [email, setEmail] = useState("alex@example.com")
  const [bio, setBio] = useState("Home owner, gadget collector. Tracking everything I own so I never lose a warranty again.")
  const [location, setLocation] = useState("San Francisco, CA")
  const [website, setWebsite] = useState("alex.example.com")

  // Password fields
  const [currentPw, setCurrentPw] = useState("")
  const [newPw, setNewPw] = useState("")
  const [confirmPw, setConfirmPw] = useState("")

  const [profileSaved, setProfileSaved] = useState(false)
  const [pwSaved, setPwSaved] = useState(false)
  const [pwError, setPwError] = useState("")

  function handleSaveProfile() {
    setProfileSaved(true)
    setTimeout(() => setProfileSaved(false), 2500)
  }

  function handleChangePassword() {
    setPwError("")
    if (!currentPw) { setPwError("Please enter your current password."); return }
    if (newPw.length < 8) { setPwError("New password must be at least 8 characters."); return }
    if (newPw !== confirmPw) { setPwError("Passwords do not match."); return }
    setPwSaved(true)
    setCurrentPw("")
    setNewPw("")
    setConfirmPw("")
    setTimeout(() => setPwSaved(false), 2500)
  }

  const pwMismatch = confirmPw.length > 0 && newPw !== confirmPw

  return (
    <div className="flex flex-col gap-0 max-w-3xl mx-auto w-full">
      {/* Sticky header */}
      <div className="sticky top-0 z-10 flex items-center gap-3 border-b border-border bg-background px-6 py-3">
        <Button variant="ghost" size="icon" className="size-8 -ml-1" onClick={onBack}>
          <ArrowLeft className="size-4" />
        </Button>
        <h1 className="text-base font-semibold">Edit Profile</h1>
      </div>

      <div className="flex flex-col gap-8 p-6">
        {/* Avatar */}
        <Section title="Photo" description="Your profile picture visible to group members.">
          <div className="flex items-center gap-4">
            <div className="relative group">
              <div className="flex size-16 items-center justify-center rounded-xl bg-muted border border-border text-xl font-bold text-primary">
                AJ
              </div>
              <button className="absolute inset-0 flex items-center justify-center rounded-xl bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity">
                <Camera className="size-4 text-white" />
              </button>
            </div>
            <div className="flex flex-col gap-1.5">
              <Button variant="outline" size="sm">Upload photo</Button>
              <p className="text-xs text-muted-foreground">JPG or PNG, max 2 MB</p>
            </div>
          </div>
        </Section>

        <Separator />

        {/* Basic info */}
        <Section title="Basic Info" description="Your name and public bio shown on your profile.">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="display-name">Display name</Label>
            <Input
              id="display-name"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder="Your full name"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="bio">Bio</Label>
            <Textarea
              id="bio"
              value={bio}
              onChange={(e) => setBio(e.target.value)}
              rows={3}
              className="resize-none"
              placeholder="A short description about yourself"
            />
            <p className="text-xs text-muted-foreground text-right">{bio.length}/200</p>
          </div>
        </Section>

        <Separator />

        {/* Contact */}
        <Section title="Contact & Location" description="Shown on your profile page.">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="email">Email address</Label>
            <Input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@example.com"
            />
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="location">Location</Label>
              <Input
                id="location"
                value={location}
                onChange={(e) => setLocation(e.target.value)}
                placeholder="City, Country"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="website">Website</Label>
              <Input
                id="website"
                value={website}
                onChange={(e) => setWebsite(e.target.value)}
                placeholder="yoursite.com"
              />
            </div>
          </div>
        </Section>

        {/* Save profile */}
        <div className="flex items-center justify-end gap-3">
          {profileSaved && (
            <span className="flex items-center gap-1.5 text-sm text-status-active">
              <CheckCircle2 className="size-4" /> Profile saved
            </span>
          )}
          <Button variant="outline" onClick={onBack}>Cancel</Button>
          <Button onClick={handleSaveProfile}>Save changes</Button>
        </div>

        <Separator />

        {/* Password */}
        <Section
          title="Change Password"
          description="Use a strong password with at least 8 characters, mixing letters, numbers, and symbols."
        >
          <PasswordInput
            id="current-pw"
            label="Current password"
            value={currentPw}
            onChange={setCurrentPw}
            placeholder="Enter current password"
          />
          <PasswordInput
            id="new-pw"
            label="New password"
            value={newPw}
            onChange={setNewPw}
            placeholder="At least 8 characters"
          />
          <PasswordStrength password={newPw} />
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="confirm-pw" className={pwMismatch ? "text-destructive" : undefined}>
              Confirm new password
            </Label>
            <div className="relative">
              <Input
                id="confirm-pw"
                type="password"
                value={confirmPw}
                onChange={(e) => setConfirmPw(e.target.value)}
                placeholder="Repeat new password"
                className={pwMismatch ? "border-destructive focus-visible:ring-destructive" : undefined}
              />
            </div>
            {pwMismatch && (
              <p className="text-xs text-destructive">Passwords do not match.</p>
            )}
          </div>

          {pwError && (
            <div className="flex items-center gap-2 rounded-lg border border-destructive/40 bg-destructive/5 px-3 py-2">
              <AlertTriangle className="size-4 text-destructive shrink-0" />
              <p className="text-sm text-destructive">{pwError}</p>
            </div>
          )}

          <div className="flex items-center justify-end gap-3">
            {pwSaved && (
              <span className="flex items-center gap-1.5 text-sm text-status-active">
                <CheckCircle2 className="size-4" /> Password updated
              </span>
            )}
            <Button
              onClick={handleChangePassword}
              disabled={!currentPw || !newPw || !confirmPw}
            >
              Update password
            </Button>
          </div>
        </Section>

        <Separator />

        {/* Danger zone */}
        <Section
          title="Danger Zone"
          description="Permanently delete your account and all associated data. This action cannot be undone."
        >
          <div className="flex items-center justify-between rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3">
            <div>
              <p className="text-sm font-medium text-destructive">Delete account</p>
              <p className="text-xs text-muted-foreground mt-0.5">All your data will be permanently removed</p>
            </div>
            <Button variant="destructive" size="sm">Delete account</Button>
          </div>
        </Section>
      </div>
    </div>
  )
}
