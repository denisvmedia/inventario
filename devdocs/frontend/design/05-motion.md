# Motion

Motion communicates causality, hierarchy, and attention. Inventario's motion language is **brisk and confident** — quick enough to feel responsive, slow enough to be perceived. No bouncy, no playful — that would conflict with the calm tone.

## Duration tokens

Five steps. Tight upper bound: nothing over 400ms in the product (hero/onboarding excluded).

```css
@theme {
  --duration-instant: 0ms;     /* state changes that should feel mechanical (radio click) */
  --duration-fast:    120ms;   /* hover, focus, color shifts, micro */
  --duration-base:    180ms;   /* default — most transitions */
  --duration-slow:    260ms;   /* dialog enter, drawer slide, page transition */
  --duration-slower:  400ms;   /* hero animations, onboarding moments */
}
```

**Rule of thumb:**
- < 100ms: feels instant; users don't notice — use for color/opacity micro-shifts
- 100–250ms: feels responsive; perceived but not waited-for — most UI motion
- 250–400ms: feels deliberate; user notices the motion — modals, drawers, choreographed sequences
- > 400ms: feels slow, in-product; reserved for delight moments only

## Easing curves

```css
@theme {
  /* Default — natural deceleration */
  --ease-default:  cubic-bezier(0.2, 0, 0, 1);

  /* Enter — element appearing */
  --ease-enter:    cubic-bezier(0, 0, 0.2, 1);

  /* Exit — element leaving */
  --ease-exit:     cubic-bezier(0.4, 0, 1, 1);

  /* Snappy — for state toggles (checkbox, switch) */
  --ease-snap:     cubic-bezier(0.4, 0, 0.2, 1);

  /* Spring-like — for delight moments (toast appearance, success confirmations) */
  --ease-spring:   cubic-bezier(0.34, 1.56, 0.64, 1);
}
```

**Easing rule of thumb:**
- Linear is forbidden except for indeterminate progress bars
- Spring/overshoot only on delight moments (3-5 places in the whole app), never on routine UI
- Defaults to `--ease-default` if you're unsure

## Motion language by component

| Component | Property | Duration | Easing |
| --- | --- | --- | --- |
| Button hover | `bg`, `color`, `box-shadow` | `fast` | `default` |
| Button press | `transform: scale(0.98)` | `fast` | `snap` |
| Card hover-elevate | `box-shadow`, `transform: translateY(-1px)` | `fast` | `default` |
| Input focus | `border-color`, `box-shadow` (focus ring) | `fast` | `default` |
| Tooltip enter | `opacity`, `transform: translateY(4px → 0)` | `fast` | `enter` |
| Tooltip exit | reverse | `fast` | `exit` |
| Toast enter | `opacity`, `transform: translateY(8px → 0) scale(0.98 → 1)` | `slow` | `spring` |
| Toast exit | `opacity`, `transform: translateY(0 → -4px)` | `base` | `exit` |
| Dialog enter | overlay `opacity`, content `opacity` + `scale(0.98 → 1)` | `slow` | `enter` |
| Dialog exit | reverse | `base` | `exit` |
| Drawer slide (mobile) | `transform: translateX(-100% → 0)` | `slow` | `enter` |
| Page transition (route change) | `opacity 0 → 1` only | `base` | `default` |
| List item enter (after creation) | `opacity 0 → 1`, `height 0 → auto` | `base` | `default` |
| List item exit (deletion) | `opacity 1 → 0`, `height auto → 0` | `base` | `exit` |
| Skeleton shimmer | `background-position` linear loop | 1500ms | linear |
| Theme toggle | `background-color`, `color` (root) | `slow` | `default` |
| Tab switch | `transform: translateY(2px)` underline + content `opacity` | `fast` | `snap` |
| Accordion open | `height 0 → auto` | `base` | `default` |
| Switch/checkbox | `transform`, `bg` | `fast` | `snap` |

## Stagger patterns

When a list of cards or items enters the screen (after data loads), stagger their entry by **30–50ms** between items, capped at the first 6 items. Items 7+ appear immediately. Without a cap, the last items in long lists arrive jarringly late.

```css
.stagger-enter > *:nth-child(1) { animation-delay: 0ms; }
.stagger-enter > *:nth-child(2) { animation-delay: 40ms; }
.stagger-enter > *:nth-child(3) { animation-delay: 80ms; }
.stagger-enter > *:nth-child(4) { animation-delay: 120ms; }
.stagger-enter > *:nth-child(5) { animation-delay: 160ms; }
.stagger-enter > *:nth-child(6) { animation-delay: 200ms; }
.stagger-enter > *:nth-child(n+7) { animation-delay: 240ms; }
```

## Perceptual loading thresholds

Tied to motion budget:

| Operation duration | UI behavior |
| --- | --- |
| < 100ms | No loading state; show result directly |
| 100–500ms | Skeleton or subtle pulse on the affected area |
| 500ms–3s | Skeleton + after 800ms a small spinner |
| 3–10s | Progress indicator (bar or count) |
| > 10s | Progress indicator + "Still working…" copy after 5s; cancel option |

**Optimistic UI:** for low-risk mutations (toggle a checkbox, mark item as draft), update UI instantly, reconcile on response. For high-risk mutations (delete, bulk delete, change status), show inline pending state with rollback on error.

## Reduced-motion strategy

Respect `prefers-reduced-motion: reduce` system-wide. Replace motion with instant transitions (or fades only):

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }

  /* Allow opacity transitions only — they're the only safe motion */
  .motion-safe-opacity {
    transition: opacity var(--duration-fast) var(--ease-default);
  }
}
```

**Reduced-motion testing:** every motion-pattern PR ships with a reduced-motion screenshot pair. If the static state is broken (e.g., element invisible without animation finishing), fix that.

## Motion budget

Per page:

- **Routine views** (list, detail, form): max 3 simultaneous motion patterns
- **Onboarding / first-load** views: up to 6 (welcome to a personality moment)
- **Background animations:** strictly forbidden (no bouncing logos, no continuous spinners on idle pages)

If you find yourself adding a fourth concurrent animation to a regular page, cut one.

## Delight moments

Three places where motion goes a step beyond utility (using `--ease-spring`):

1. **First successful inventory entry** (onboarding completion): subtle confetti + scale-up of the new card, ~400ms
2. **Backup/export complete:** toast slides in with spring, success icon scales from 0.6 → 1.1 → 1
3. **Empty state → first item:** the empty state crossfades + the item drops in from above

Reserved. Do not add more without a designer's eye on the whole app.

## Hover-only motions for keyboard users

Replace `:hover` reliance with `:hover, :focus-visible` so keyboard users get the same affordances. Then the motion design works for both input modes.

## Anti-patterns

- ❌ Spinners that show on every fetch under 200ms (causes flicker)
- ❌ Bouncy easings on professional-context UI (forms, tables)
- ❌ Page-load animations on every navigation (annoying after the third time)
- ❌ Hover effects that move layout (causes click-target jitter — use `transform`, not margin/padding)
- ❌ Toasts that auto-dismiss faster than read time (min 4s for success, 6s for warning)
- ❌ Loading skeletons that don't match the eventual content shape (creates layout shift)
- ❌ Independent animations on each card (use stagger or none)
- ❌ Sound effects (we are not building a video game)

## Vue Router transitions

Default page transition: `opacity` only, `--duration-base`, `--ease-default`. No slide, no scale.

```vue
<router-view v-slot="{ Component, route }">
  <transition
    name="page"
    mode="out-in"
    appear
  >
    <component :is="Component" :key="route.path" />
  </transition>
</router-view>
```

```css
.page-enter-active, .page-leave-active {
  transition: opacity var(--duration-base) var(--ease-default);
}
.page-enter-from, .page-leave-to { opacity: 0; }
```

Subtle transitions feel modern; sliding pages feel like a 2014 mobile app.
