import { useState } from "react"
import { Package, Eye, EyeOff, ArrowRight, ArrowLeft, Mail, Lock, User, CircleCheck as CheckCircle2, Circle as XCircle, Clock, Building2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import { Checkbox } from "@/components/ui/checkbox"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

type AuthMode = "login" | "register" | "forgot-password" | "forgot-sent" | "reset-password" | "reset-done" | "verify-email" | "invite"

// Simulated URL params for demo
const DEMO_PARAMS = {
  resetToken: "demo-reset-token-abc123",
  verifyToken: "demo-verify-token-xyz789",
  verifyStatus: "success" as "success" | "expired" | "invalid",
  invite: {
    token: "inv-abc123",
    groupName: "Main Residence",
    groupDescription: "Primary home at 14 Oak Street",
    invitedBy: "Alex Johnson",
    role: "user" as "admin" | "user",
    inviteeEmail: "newuser@example.com",
    hasAccount: false,
  },
}

interface AuthViewProps {
  onAuth: () => void
}

export function AuthView({ onAuth }: AuthViewProps) {
  const [mode, setMode] = useState<AuthMode>("login")
  const [showPassword, setShowPassword] = useState(false)
  const [showConfirm, setShowConfirm] = useState(false)

  const DecorativePanel = () => (
    <div className="hidden lg:flex lg:w-[44%] bg-primary flex-col justify-between p-10 relative overflow-hidden">
      <div
        className="absolute inset-0 opacity-[0.04]"
        style={{
          backgroundImage:
            "repeating-linear-gradient(0deg,transparent,transparent 39px,currentColor 39px,currentColor 40px),repeating-linear-gradient(90deg,transparent,transparent 39px,currentColor 39px,currentColor 40px)",
        }}
      />
      <div className="relative z-10 flex items-center gap-2.5">
        <div className="flex size-8 items-center justify-center rounded-lg bg-primary-foreground/10">
          <Package className="size-4 text-primary-foreground" />
        </div>
        <span className="text-lg font-semibold text-primary-foreground">Inventario</span>
      </div>
      <div className="relative z-10 space-y-4">
        <blockquote className="text-2xl font-semibold leading-snug text-primary-foreground">
          "Everything I own, where I can find it — warranties, values, and all the parts I keep forgetting to order."
        </blockquote>
        <div className="flex items-center gap-3">
          <div className="size-9 rounded-full bg-primary-foreground/15" />
          <div>
            <p className="text-sm font-medium text-primary-foreground">Personal inventory</p>
            <p className="text-xs text-primary-foreground/60">for real life</p>
          </div>
        </div>
      </div>
      <div className="relative z-10 flex gap-3">
        {[
          { label: "Items tracked", value: "8" },
          { label: "Warranties active", value: "3" },
          { label: "Est. value", value: "$6,340" },
        ].map((s) => (
          <div
            key={s.label}
            className="flex-1 rounded-xl bg-primary-foreground/8 border border-primary-foreground/12 p-3 backdrop-blur-sm"
          >
            <p className="text-xl font-bold text-primary-foreground">{s.value}</p>
            <p className="text-[11px] text-primary-foreground/60 mt-0.5">{s.label}</p>
          </div>
        ))}
      </div>
    </div>
  )

  const MobileLogo = () => (
    <div className="mb-8 flex items-center gap-2 lg:hidden">
      <div className="flex size-7 items-center justify-center rounded-md bg-primary">
        <Package className="size-4 text-primary-foreground" />
      </div>
      <span className="text-base font-semibold">Inventario</span>
    </div>
  )

  const isFullscreenForm = mode === "verify-email" || mode === "invite"

  if (isFullscreenForm) {
    return (
      <div className="flex min-h-svh w-full">
        <DecorativePanel />
        <div className="flex flex-1 flex-col items-center justify-center bg-background px-6 py-12">
          <MobileLogo />
          <div className="w-full max-w-sm">
            {mode === "verify-email" && (
              <VerifyEmailScreen status={DEMO_PARAMS.verifyStatus} onGoToLogin={() => setMode("login")} />
            )}
            {mode === "invite" && (
              <InviteAcceptScreen
                invite={DEMO_PARAMS.invite}
                onAccept={onAuth}
                onDecline={() => setMode("login")}
                showPassword={showPassword}
                showConfirm={showConfirm}
                onTogglePassword={() => setShowPassword((v) => !v)}
                onToggleConfirm={() => setShowConfirm((v) => !v)}
              />
            )}
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="flex min-h-svh w-full">
      <DecorativePanel />
      <div className="flex flex-1 flex-col items-center justify-center bg-background px-6 py-12">
        <MobileLogo />
        <div className="w-full max-w-sm">
          {/* Demo mode switcher */}
          <div className="mb-4 flex flex-wrap gap-1.5 p-2 rounded-lg bg-muted/50 border border-border">
            <p className="w-full text-[10px] font-medium text-muted-foreground px-1 mb-0.5">Demo screens:</p>
            {(["login", "register", "forgot-password", "forgot-sent", "reset-password", "reset-done", "verify-email", "invite"] as AuthMode[]).map((m) => (
              <button
                key={m}
                onClick={() => setMode(m)}
                className={cn(
                  "px-2 py-0.5 rounded text-[10px] font-medium transition-colors",
                  mode === m ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground"
                )}
              >
                {m}
              </button>
            ))}
          </div>

          {mode === "login" && (
            <LoginForm
              showPassword={showPassword}
              onTogglePassword={() => setShowPassword((v) => !v)}
              onAuth={onAuth}
              onSwitchMode={() => setMode("register")}
              onForgotPassword={() => setMode("forgot-password")}
            />
          )}
          {mode === "register" && (
            <RegisterForm
              showPassword={showPassword}
              showConfirm={showConfirm}
              onTogglePassword={() => setShowPassword((v) => !v)}
              onToggleConfirm={() => setShowConfirm((v) => !v)}
              onAuth={onAuth}
              onSwitchMode={() => setMode("login")}
            />
          )}
          {mode === "forgot-password" && (
            <ForgotPasswordForm
              onSent={() => setMode("forgot-sent")}
              onBack={() => setMode("login")}
            />
          )}
          {mode === "forgot-sent" && (
            <ForgotPasswordSent onBack={() => setMode("login")} />
          )}
          {mode === "reset-password" && (
            <ResetPasswordForm
              showPassword={showPassword}
              showConfirm={showConfirm}
              onTogglePassword={() => setShowPassword((v) => !v)}
              onToggleConfirm={() => setShowConfirm((v) => !v)}
              onDone={() => setMode("reset-done")}
            />
          )}
          {mode === "reset-done" && (
            <ResetDone onGoToLogin={() => setMode("login")} />
          )}
        </div>
      </div>
    </div>
  )
}

// ─── Login ──────────────────────────────────────────────────────────────────

interface LoginFormProps {
  showPassword: boolean
  onTogglePassword: () => void
  onAuth: () => void
  onSwitchMode: () => void
  onForgotPassword: () => void
}

function LoginForm({ showPassword, onTogglePassword, onAuth, onSwitchMode, onForgotPassword }: LoginFormProps) {
  return (
    <div className="space-y-6">
      <div className="space-y-1.5">
        <h1 className="text-2xl font-semibold tracking-tight">Welcome back</h1>
        <p className="text-sm text-muted-foreground">Sign in to your inventory</p>
      </div>

      <div className="space-y-4">
        <div className="space-y-1.5">
          <Label htmlFor="email" className="text-sm font-medium">Email</Label>
          <div className="relative">
            <Mail className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input id="email" type="email" placeholder="you@example.com" className="pl-9" defaultValue="alex@example.com" />
          </div>
        </div>

        <div className="space-y-1.5">
          <div className="flex items-center justify-between">
            <Label htmlFor="password" className="text-sm font-medium">Password</Label>
            <button
              type="button"
              onClick={onForgotPassword}
              className="text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              Forgot password?
            </button>
          </div>
          <div className="relative">
            <Lock className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input id="password" type={showPassword ? "text" : "password"} placeholder="••••••••" className="pl-9 pr-9" defaultValue="password" />
            <button type="button" onClick={onTogglePassword} className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors">
              {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
            </button>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <Checkbox id="remember" />
          <Label htmlFor="remember" className="text-sm font-normal text-muted-foreground cursor-pointer">Remember me for 30 days</Label>
        </div>
      </div>

      <Button className="w-full gap-2" onClick={onAuth}>
        Sign in
        <ArrowRight className="size-4" />
      </Button>

      <div className="relative">
        <Separator />
        <span className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 bg-background px-2 text-xs text-muted-foreground">
          or continue with
        </span>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <Button variant="outline" className="gap-2 text-sm">
          <svg className="size-4" viewBox="0 0 24 24" fill="currentColor">
            <path d="M12.48 10.92v3.28h7.84c-.24 1.84-.853 3.187-1.787 4.133-1.147 1.147-2.933 2.4-6.053 2.4-4.827 0-8.6-3.893-8.6-8.72s3.773-8.72 8.6-8.72c2.6 0 4.507 1.027 5.907 2.347l2.307-2.307C18.747 1.44 16.133 0 12.48 0 5.867 0 .307 5.387.307 12s5.56 12 12.173 12c3.573 0 6.267-1.173 8.373-3.36 2.16-2.16 2.84-5.213 2.84-7.667 0-.76-.053-1.467-.173-2.053H12.48z" />
          </svg>
          Google
        </Button>
        <Button variant="outline" className="gap-2 text-sm">
          <svg className="size-4" viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12" />
          </svg>
          GitHub
        </Button>
      </div>

      <p className="text-center text-sm text-muted-foreground">
        Don't have an account?{" "}
        <button className="font-medium text-foreground hover:underline underline-offset-4" onClick={onSwitchMode}>
          Create one
        </button>
      </p>
    </div>
  )
}

// ─── Register ───────────────────────────────────────────────────────────────

interface RegisterFormProps {
  showPassword: boolean
  showConfirm: boolean
  onTogglePassword: () => void
  onToggleConfirm: () => void
  onAuth: () => void
  onSwitchMode: () => void
}

function RegisterForm({ showPassword, showConfirm, onTogglePassword, onToggleConfirm, onAuth, onSwitchMode }: RegisterFormProps) {
  return (
    <div className="space-y-6">
      <div className="space-y-1.5">
        <h1 className="text-2xl font-semibold tracking-tight">Create account</h1>
        <p className="text-sm text-muted-foreground">Start tracking your inventory for free</p>
      </div>

      <div className="space-y-4">
        <div className="space-y-1.5">
          <Label htmlFor="name" className="text-sm font-medium">Full name</Label>
          <div className="relative">
            <User className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input id="name" placeholder="Alex Johnson" className="pl-9" />
          </div>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="reg-email" className="text-sm font-medium">Email</Label>
          <div className="relative">
            <Mail className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input id="reg-email" type="email" placeholder="you@example.com" className="pl-9" />
          </div>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="reg-password" className="text-sm font-medium">Password</Label>
          <div className="relative">
            <Lock className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input id="reg-password" type={showPassword ? "text" : "password"} placeholder="At least 8 characters" className="pl-9 pr-9" />
            <button type="button" onClick={onTogglePassword} className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors">
              {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
            </button>
          </div>
          <div className="flex gap-1 mt-1.5">
            {[1, 2, 3, 4].map((i) => (
              <div key={i} className={`h-1 flex-1 rounded-full ${i <= 2 ? "bg-status-expiring" : "bg-muted"}`} />
            ))}
          </div>
          <p className="text-xs text-muted-foreground">Use 8+ characters with letters and numbers</p>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="confirm" className="text-sm font-medium">Confirm password</Label>
          <div className="relative">
            <Lock className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input id="confirm" type={showConfirm ? "text" : "password"} placeholder="Repeat your password" className="pl-9 pr-9" />
            <button type="button" onClick={onToggleConfirm} className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors">
              {showConfirm ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
            </button>
          </div>
        </div>

        <div className="flex items-start gap-2 pt-1">
          <Checkbox id="terms" className="mt-0.5" />
          <Label htmlFor="terms" className="text-sm font-normal text-muted-foreground leading-relaxed cursor-pointer">
            I agree to the{" "}
            <span className="text-foreground font-medium hover:underline underline-offset-4 cursor-pointer">Terms of Service</span>
            {" "}and{" "}
            <span className="text-foreground font-medium hover:underline underline-offset-4 cursor-pointer">Privacy Policy</span>
          </Label>
        </div>
      </div>

      <Button className="w-full gap-2" onClick={onAuth}>
        Create account
        <ArrowRight className="size-4" />
      </Button>

      <p className="text-center text-sm text-muted-foreground">
        Already have an account?{" "}
        <button className="font-medium text-foreground hover:underline underline-offset-4" onClick={onSwitchMode}>
          Sign in
        </button>
      </p>
    </div>
  )
}

// ─── Forgot password ────────────────────────────────────────────────────────

function ForgotPasswordForm({ onSent, onBack }: { onSent: () => void; onBack: () => void }) {
  const [email, setEmail] = useState("")
  return (
    <div className="space-y-6">
      <button onClick={onBack} className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors">
        <ArrowLeft className="size-4" />
        Back to sign in
      </button>
      <div className="space-y-1.5">
        <h1 className="text-2xl font-semibold tracking-tight">Forgot password?</h1>
        <p className="text-sm text-muted-foreground">
          Enter your email and we'll send you a link to reset your password.
        </p>
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="forgot-email" className="text-sm font-medium">Email address</Label>
        <div className="relative">
          <Mail className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            id="forgot-email"
            type="email"
            placeholder="you@example.com"
            className="pl-9"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoFocus
          />
        </div>
      </div>
      <Button className="w-full gap-2" onClick={onSent} disabled={!email.trim()}>
        Send reset link
        <ArrowRight className="size-4" />
      </Button>
    </div>
  )
}

function ForgotPasswordSent({ onBack }: { onBack: () => void }) {
  return (
    <div className="space-y-6 text-center">
      <div className="flex justify-center">
        <div className="flex size-16 items-center justify-center rounded-full bg-primary/10">
          <Mail className="size-8 text-primary" />
        </div>
      </div>
      <div className="space-y-1.5">
        <h1 className="text-2xl font-semibold tracking-tight">Check your email</h1>
        <p className="text-sm text-muted-foreground">
          We sent a password reset link to <span className="font-medium text-foreground">you@example.com</span>.
          The link expires in 1 hour.
        </p>
      </div>
      <div className="rounded-lg border border-border bg-muted/30 p-4 text-left space-y-2">
        <p className="text-xs font-medium text-muted-foreground">Didn't get the email?</p>
        <ul className="text-xs text-muted-foreground space-y-1 list-disc list-inside">
          <li>Check your spam folder</li>
          <li>Make sure you entered the right address</li>
        </ul>
        <Button variant="outline" size="sm" className="w-full mt-1">Resend email</Button>
      </div>
      <button onClick={onBack} className="text-sm text-muted-foreground hover:text-foreground transition-colors underline underline-offset-4">
        Back to sign in
      </button>
    </div>
  )
}

// ─── Reset password ─────────────────────────────────────────────────────────

function ResetPasswordForm({ showPassword, showConfirm, onTogglePassword, onToggleConfirm, onDone }: {
  showPassword: boolean; showConfirm: boolean
  onTogglePassword: () => void; onToggleConfirm: () => void
  onDone: () => void
}) {
  const [password, setPassword] = useState("")
  const [confirm, setConfirm] = useState("")
  const match = password.length >= 8 && password === confirm

  return (
    <div className="space-y-6">
      <div className="space-y-1.5">
        <h1 className="text-2xl font-semibold tracking-tight">Set new password</h1>
        <p className="text-sm text-muted-foreground">Choose a strong password for your account.</p>
      </div>
      <div className="space-y-4">
        <div className="space-y-1.5">
          <Label htmlFor="new-password" className="text-sm font-medium">New password</Label>
          <div className="relative">
            <Lock className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              id="new-password"
              type={showPassword ? "text" : "password"}
              placeholder="At least 8 characters"
              className="pl-9 pr-9"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoFocus
            />
            <button type="button" onClick={onTogglePassword} className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors">
              {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
            </button>
          </div>
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="confirm-password" className="text-sm font-medium">Confirm password</Label>
          <div className="relative">
            <Lock className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              id="confirm-password"
              type={showConfirm ? "text" : "password"}
              placeholder="Repeat new password"
              className={cn("pl-9 pr-9", confirm.length > 0 && (match ? "border-status-active" : "border-status-expired"))}
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
            />
            <button type="button" onClick={onToggleConfirm} className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors">
              {showConfirm ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
            </button>
          </div>
          {confirm.length > 0 && !match && (
            <p className="text-xs text-status-expired">Passwords don't match</p>
          )}
        </div>
      </div>
      <Button className="w-full gap-2" onClick={onDone} disabled={!match}>
        Reset password
        <ArrowRight className="size-4" />
      </Button>
    </div>
  )
}

function ResetDone({ onGoToLogin }: { onGoToLogin: () => void }) {
  return (
    <div className="space-y-6 text-center">
      <div className="flex justify-center">
        <div className="flex size-16 items-center justify-center rounded-full bg-status-active/10">
          <CheckCircle2 className="size-8 text-status-active" />
        </div>
      </div>
      <div className="space-y-1.5">
        <h1 className="text-2xl font-semibold tracking-tight">Password updated</h1>
        <p className="text-sm text-muted-foreground">Your password has been reset. You can now sign in with your new password.</p>
      </div>
      <Button className="w-full gap-2" onClick={onGoToLogin}>
        Sign in
        <ArrowRight className="size-4" />
      </Button>
    </div>
  )
}

// ─── Verify email ───────────────────────────────────────────────────────────

function VerifyEmailScreen({ status, onGoToLogin }: { status: "success" | "expired" | "invalid"; onGoToLogin: () => void }) {
  const configs = {
    success: {
      icon: CheckCircle2,
      iconClass: "text-status-active",
      bgClass: "bg-status-active/10",
      title: "Email verified!",
      message: "Your email address has been confirmed. You're all set to use Inventario.",
      action: "Continue to app",
    },
    expired: {
      icon: Clock,
      iconClass: "text-status-expiring",
      bgClass: "bg-status-expiring/10",
      title: "Link expired",
      message: "This verification link has expired. Links are valid for 24 hours.",
      action: "Request new link",
    },
    invalid: {
      icon: XCircle,
      iconClass: "text-status-expired",
      bgClass: "bg-status-expired/10",
      title: "Invalid link",
      message: "This verification link is not valid or has already been used.",
      action: "Back to sign in",
    },
  }
  const c = configs[status]
  return (
    <div className="space-y-6 text-center">
      <div className="flex justify-center">
        <div className={cn("flex size-16 items-center justify-center rounded-full", c.bgClass)}>
          <c.icon className={cn("size-8", c.iconClass)} />
        </div>
      </div>
      <div className="space-y-1.5">
        <h1 className="text-2xl font-semibold tracking-tight">{c.title}</h1>
        <p className="text-sm text-muted-foreground">{c.message}</p>
      </div>
      <Button className="w-full gap-2" onClick={onGoToLogin}>
        {c.action}
        <ArrowRight className="size-4" />
      </Button>
    </div>
  )
}

// ─── Invite acceptance ──────────────────────────────────────────────────────

interface InviteInfo {
  token: string
  groupName: string
  groupDescription: string
  invitedBy: string
  role: "admin" | "user"
  inviteeEmail: string
  hasAccount: boolean
}

function InviteAcceptScreen({
  invite,
  onAccept,
  onDecline,
  showPassword,
  showConfirm,
  onTogglePassword,
  onToggleConfirm,
}: {
  invite: InviteInfo
  onAccept: () => void
  onDecline: () => void
  showPassword: boolean
  showConfirm: boolean
  onTogglePassword: () => void
  onToggleConfirm: () => void
}) {
  const [accepted, setAccepted] = useState(false)

  if (accepted && !invite.hasAccount) {
    // Show registration form
    return (
      <div className="space-y-6">
        <button onClick={() => setAccepted(false)} className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors">
          <ArrowLeft className="size-4" />
          Back to invite
        </button>
        <div className="space-y-1.5">
          <h1 className="text-2xl font-semibold tracking-tight">Create your account</h1>
          <p className="text-sm text-muted-foreground">
            Set a password to complete joining <span className="font-medium text-foreground">{invite.groupName}</span>.
          </p>
        </div>
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="inv-name" className="text-sm font-medium">Full name</Label>
            <div className="relative">
              <User className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input id="inv-name" placeholder="Your name" className="pl-9" />
            </div>
          </div>
          <div className="space-y-1.5">
            <Label className="text-sm font-medium">Email</Label>
            <Input value={invite.inviteeEmail} disabled className="opacity-70" />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="inv-password" className="text-sm font-medium">Password</Label>
            <div className="relative">
              <Lock className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input id="inv-password" type={showPassword ? "text" : "password"} placeholder="At least 8 characters" className="pl-9 pr-9" />
              <button type="button" onClick={onTogglePassword} className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors">
                {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
              </button>
            </div>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="inv-confirm" className="text-sm font-medium">Confirm password</Label>
            <div className="relative">
              <Lock className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input id="inv-confirm" type={showConfirm ? "text" : "password"} placeholder="Repeat password" className="pl-9 pr-9" />
              <button type="button" onClick={onToggleConfirm} className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors">
                {showConfirm ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
              </button>
            </div>
          </div>
        </div>
        <Button className="w-full gap-2" onClick={onAccept}>
          Join {invite.groupName}
          <ArrowRight className="size-4" />
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="space-y-1.5">
        <h1 className="text-2xl font-semibold tracking-tight">You're invited!</h1>
        <p className="text-sm text-muted-foreground">
          <span className="font-medium text-foreground">{invite.invitedBy}</span> invited you to join their inventory group.
        </p>
      </div>

      {/* Group card */}
      <div className="rounded-xl border border-border bg-card p-4 space-y-3">
        <div className="flex items-center gap-3">
          <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10 shrink-0">
            <Building2 className="size-5 text-primary" />
          </div>
          <div className="min-w-0">
            <p className="font-semibold text-sm">{invite.groupName}</p>
            <p className="text-xs text-muted-foreground truncate">{invite.groupDescription}</p>
          </div>
        </div>
        <Separator />
        <div className="flex items-center justify-between">
          <p className="text-xs text-muted-foreground">Your role</p>
          <Badge variant="secondary" className="text-xs capitalize">{invite.role}</Badge>
        </div>
        <div className="flex items-center justify-between">
          <p className="text-xs text-muted-foreground">Invited as</p>
          <p className="text-xs font-medium">{invite.inviteeEmail}</p>
        </div>
      </div>

      <div className="flex flex-col gap-2">
        {invite.hasAccount ? (
          <Button className="w-full gap-2" onClick={onAccept}>
            Accept invitation
            <ArrowRight className="size-4" />
          </Button>
        ) : (
          <Button className="w-full gap-2" onClick={() => setAccepted(true)}>
            Accept & create account
            <ArrowRight className="size-4" />
          </Button>
        )}
        <Button variant="outline" className="w-full" onClick={onDecline}>
          Decline invitation
        </Button>
      </div>

      {!invite.hasAccount && (
        <p className="text-center text-xs text-muted-foreground">
          Already have an account?{" "}
          <button className="font-medium text-foreground hover:underline underline-offset-4" onClick={onAccept}>
            Sign in to accept
          </button>
        </p>
      )}
    </div>
  )
}
