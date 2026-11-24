import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { cn } from "@/lib/utils"
import { userInitials } from "@/lib/user-initials"

interface UserAvatarProps {
  name?: string | null
  email?: string | null
  // Signed URL of the profile photo (`avatar_url` on /auth/me and on each
  // member). Absent, or failing to load, falls back to the initials.
  src?: string | null
  // Shape, size and border of the tile. Every surface draws the avatar its
  // own way in the mock (a 32px circle in lists, a 72px rounded square on the
  // profile), so the call site owns those classes; the image is clipped to
  // whatever shape the root gets.
  className?: string
  // Colors and type of the initials, which also differ per surface.
  fallbackClassName?: string
  testId?: string
}

// UserAvatar is the one place a user is drawn: the sidebar's user menu, the
// account card, the profile pages, the members list and the admin views all
// render this, so the photo-or-initials rule cannot drift between them.
//
// The tile is decorative: every call site prints the user's name next to it,
// so it is hidden from assistive technology rather than announced twice.
export function UserAvatar({
  name,
  email,
  src,
  className,
  fallbackClassName,
  testId,
}: UserAvatarProps) {
  return (
    <Avatar className={className} aria-hidden="true" data-testid={testId}>
      {src ? (
        <AvatarImage src={src} alt="" className="object-cover" data-testid="user-avatar-image" />
      ) : null}
      {/* rounded-[inherit]: the primitive's fallback is a circle, which would
          show as a disc inside the square tiles the profile pages use. */}
      <AvatarFallback
        className={cn("rounded-[inherit] font-semibold", fallbackClassName)}
        data-testid="user-avatar-initials"
      >
        {userInitials(name, email)}
      </AvatarFallback>
    </Avatar>
  )
}
