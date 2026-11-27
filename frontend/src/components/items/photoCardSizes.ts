// PHOTO_CARD_IMAGE_SIZES is the `sizes` hint of a photo card's image: the
// width the image box is drawn at, for the browser to choose a thumbnail
// (150, 300 or 600 px wide) from (#2797, #2900). It follows the layout with
// the sidebar open, measured in Chromium:
//
//   - below 640 px: no sidebar, the shell's and the page's `p-6` (96 px) and
//     two columns with a 16 px gap; the card border takes 2 px.
//   - 640 to 767 px: the shell's `container` caps the page at 640 px, so the
//     card stays 262 px.
//   - from 768 px: the 16rem sidebar as well (352 px in all), and the columns
//     of CommodityPhotoGrid's container queries: 2, then 3 from a 576 px
//     grid (928 px window), 4 from 768 (1120), 5 from 1024 (1376) and 6 from
//     1152 (1504), until the page stops growing at 1584 px.
//
// Each band is (window - 352 - gaps) / columns - 2. A collapsed sidebar or a
// classic scrollbar moves the card a few pixels from the hint, which does not
// change the thumbnail chosen. Change the hint with the grid, the page
// padding or the sidebar width.
export const PHOTO_CARD_IMAGE_SIZES = [
  "(min-width: 1584px) 190px",
  "(min-width: 1504px) calc(16.67vw - 74px)",
  "(min-width: 1376px) calc(20vw - 85px)",
  "(min-width: 1120px) calc(25vw - 102px)",
  "(min-width: 928px) calc(33.33vw - 130px)",
  "(min-width: 768px) calc(50vw - 186px)",
  "(min-width: 640px) 262px",
  "calc(50vw - 58px)",
].join(", ")
