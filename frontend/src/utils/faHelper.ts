/**
 * Helper functions for migrating from Font Awesome CSS classes to component-based usage
 */

/**
 * Converts a Font Awesome class string to an icon name for use with font-awesome-icon component
 * 
 * Example: 
 * - Input: "fas fa-file-pdf"
 * - Output: "file-pdf"
 * 
 * @param faClass Font Awesome class string (e.g., "fas fa-file-pdf")
 * @returns Icon name without the prefix (e.g., "file-pdf")
 */
export const faClassToIcon = (faClass: string): string => {
  // Handle empty or invalid input
  if (!faClass) return '';
  
  // Extract the icon name from the class string
  // This handles both "fas fa-icon-name" and "fa-icon-name" formats
  const match = faClass.match(/fa[srlbd]?\s+fa-([a-z0-9-]+)/i) || faClass.match(/fa-([a-z0-9-]+)/i);
  
  if (match && match[1]) {
    return match[1];
  }
  
  // If no match found, return the original string
  return faClass;
};

/**
 * Extracts size information from a Font Awesome class string
 * 
 * Example:
 * - Input: "fas fa-file-pdf fa-2x"
 * - Output: "2x"
 * 
 * @param faClass Font Awesome class string (e.g., "fas fa-file-pdf fa-2x")
 * @returns Size string or undefined if no size specified
 */
export const faClassToSize = (faClass: string): string | undefined => {
  const match = faClass.match(/fa-([1-9][0-9]?x)/i);
  return match ? match[1] : undefined;
};

/**
 * Checks if a Font Awesome class includes a specific style
 * 
 * @param faClass Font Awesome class string
 * @param style Style to check for (e.g., "spin", "pulse", "flip-horizontal")
 * @returns True if the style is present
 */
export const faClassHasStyle = (faClass: string, style: string): boolean => {
  return faClass.includes(`fa-${style}`);
};
