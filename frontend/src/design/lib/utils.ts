import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * Merge class names, resolving Tailwind conflicts via tailwind-merge.
 *
 * Used by every shadcn-vue component we copy in and by any pattern that
 * needs to compose variant classes with a caller-supplied `class` prop.
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs))
}
