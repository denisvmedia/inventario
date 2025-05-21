# SCSS Duplicates Analysis

This document provides an analysis of similar but not identical SCSS styles found across the Vue components in the project. These styles have not been moved to `_base.scss` because they have slight variations that require manual review.

## Similar Form Styles

### Form Container Styles
- `.inline-form` in AreaForm.vue and LocationForm.vue:
  - Both have similar styles but with different margin values
  - AreaForm.vue has `margin-left: 2rem` for hierarchy indentation
  - LocationForm.vue has `margin-bottom: 1.5rem` instead of `1rem`

### Form Actions
- `.form-actions` has different gap values:
  - In form components: `gap: 0.5rem`
  - In view components: `gap: 1rem`
  - In CommodityForm.vue: `margin-top: 2rem` instead of `1rem`

### Form Error
- `.form-error` has different styles:
  - In AreaForm.vue and LocationForm.vue: Uses `rgba($danger-color, 0.1)` for background
  - In CommodityEditView.vue: Uses `#f8d7da` for background and has `$error-text-color` for text

## Container Styles

### Card-like Containers
- `.info-card` in CommodityDetailView.vue
- `.form` in CommodityCreateView.vue
Both have similar box-shadow and border-radius styles but different padding and internal structure.

## Recommendations

1. Consider standardizing the following:
   - Gap sizes in `.form-actions`
   - Margin values in form containers
   - Error message styling

2. For component-specific styles (like the indentation in AreaForm), keep these separate from the base styles.

3. Consider creating additional shared style classes in `_components.scss` for card-like containers that have similar but not identical styles.
