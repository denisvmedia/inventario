# Font Awesome Migration Guide

This document provides guidance on migrating from the CDN-based Font Awesome to the component-based approach.

## Why Migrate?

- **Better tree-shaking**: Only the icons you use are included in the bundle
- **TypeScript support**: Better type checking and autocompletion
- **More control**: Easier to manipulate icons programmatically
- **Consistent API**: Use the same component API throughout the application

## How to Use Font Awesome in Vue Components

### Basic Usage

Replace this:
```html
<i class="fas fa-user"></i>
```

With this:
```html
<font-awesome-icon icon="user" />
```

### Using Different Icon Styles (Solid, Regular, Brands)

For solid icons (default):
```html
<font-awesome-icon icon="user" />
```

For regular icons:
```html
<font-awesome-icon :icon="['far', 'user']" />
```

For brand icons:
```html
<font-awesome-icon :icon="['fab', 'github']" />
```

### Sizing

Replace this:
```html
<i class="fas fa-user fa-2x"></i>
```

With this:
```html
<font-awesome-icon icon="user" size="2x" />
```

Available sizes: `xs`, `sm`, `lg`, `1x`, `2x`, `3x`, `4x`, `5x`, `6x`, `7x`, `8x`, `9x`, `10x`

### Transformations

```html
<font-awesome-icon icon="user" rotation="90" />
<font-awesome-icon icon="user" flip="horizontal" />
<font-awesome-icon icon="user" flip="vertical" />
<font-awesome-icon icon="user" flip="both" />
```

### Animations

```html
<font-awesome-icon icon="spinner" spin />
<font-awesome-icon icon="spinner" pulse />
```

### Fixed Width

```html
<font-awesome-icon icon="user" fixed-width />
```

## Adding New Icons

If you need to use an icon that's not already imported, add it to the `fontawesome.ts` file:

```typescript
// src/fontawesome.ts
import { library } from '@fortawesome/fontawesome-svg-core'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'

// Import the icons you need
import { faUser, faCoffee } from '@fortawesome/free-solid-svg-icons'
import { faGithub } from '@fortawesome/free-brands-svg-icons'
import { faUser as farUser } from '@fortawesome/free-regular-svg-icons'

// Add icons to the library
library.add(faUser, faCoffee, faGithub, farUser)

export { FontAwesomeIcon }
```

## Helper Functions

We've created helper functions to make migration easier:

```typescript
import { faClassToIcon, faClassToSize } from '@/utils/faHelper'

// Convert old class to icon name
const iconName = faClassToIcon('fas fa-file-pdf')  // Returns 'file-pdf'

// Extract size from class
const size = faClassToSize('fas fa-file-pdf fa-2x')  // Returns '2x'
```

## Common Migration Patterns

### From a static icon:

```html
<!-- Before -->
<i class="fas fa-user"></i>

<!-- After -->
<font-awesome-icon icon="user" />
```

### From a dynamic icon:

```html
<!-- Before -->
<i :class="getIconClass()"></i>

<!-- After -->
<font-awesome-icon :icon="getIconName()" />
```

Update your methods to return just the icon name instead of the full class:

```typescript
// Before
const getIconClass = () => {
  return 'fas fa-file-pdf'
}

// After
const getIconName = () => {
  return 'file-pdf'
}
```

### From a conditional icon:

```html
<!-- Before -->
<i :class="isActive ? 'fas fa-check' : 'fas fa-times'"></i>

<!-- After -->
<font-awesome-icon :icon="isActive ? 'check' : 'times'" />
```
