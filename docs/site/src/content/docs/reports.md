---
title: Reports
description: Generate a printable insurance report for a single item or a whole location, then print it or save it as a PDF.
---

Inventario can turn your inventory into a clean, printable report. The **Insurance Report** is built for exactly the moment you'd dread otherwise — showing an insurer (or yourself) what you own, what it's worth, and which warranties are still in force. You can produce a report for a **single item** or for an **entire location**, then print it or save it as a PDF.

![The Reports landing page with the Insurance Report card](../../assets/screenshots/reports-landing.png)

## Open the Reports section

1. Click **Reports** in the sidebar.
2. The Reports page lists the kinds of report you can generate. Today there's one: **Insurance Report**.
3. Click the **Insurance Report** card to open it.

## The Insurance Report

The report opens with a toolbar at the top and the report "sheet" below it. The toolbar is where you choose what the report covers — it never appears on the printed page, so only the report itself lands on paper.

In the toolbar you'll find:

- A **back arrow** (**Back to reports**) to return to the Reports landing page.
- A **mode toggle** with two buttons: **Item** and **Location**.
- A **selector** for picking which item or location to report on.
- A **photo size** toggle: **Thumbnails** or **Full size**.
- A **Print / Save PDF** button.

Every report sheet ends with a footer showing your group name and the date the report was generated.

## Single-item report

Use this when you need a detailed write-up of one thing — a laptop, a watch, a bike.

1. In the toolbar, choose **Item**.
2. Pick the thing you want from the **Select item…** dropdown.
3. The report fills in with everything Inventario knows about that item.

![A single-item insurance report showing value cards, item details, warranty status, and photos](../../assets/screenshots/insurance-item.png)

A single-item report shows:

- **A header** with the item's name (and its short name, if different), labelled **Item Insurance Report**, plus the date it was generated.
- **Two value cards:**
  - **Purchase Price** — what you paid, in the currency you recorded the purchase in. The purchase date appears beneath it if you set one.
  - **Estimated Value** — the current replacement estimate, in your group's currency.
- **Item Details** — **Type**, **Serial Number**, **Purchase Date**, and **Location** (shown as "Location · Area").
- **Warranty Status** — a coloured badge reading **Active**, **Expiring Soon**, **Expired**, or **No Warranty**, with the expiry date and any warranty notes you've added.
- **Photographs** — the image files attached to the item (see [Files & photos](../files-and-photos/)).
- **Notes** — the item's comments, if it has any.

:::note
Any field you haven't filled in shows a dash (—) rather than being left blank, so the report always reads cleanly.
:::

:::tip
To get the most out of a single-item report, fill in the item's price, purchase date, serial number, and warranty details, and attach a clear photo or two. See [Items](../items/) and [Warranties, loans & maintenance](../warranties-loans-maintenance/).
:::

## Whole-location report

Use this when you want one document covering everything in a place — a flat, a garage, a storage unit. This is the **default** view when you first open the report.

1. In the toolbar, choose **Location**.
2. Pick the place from the **Select location…** dropdown.
3. The report lists every item stored in that location's areas, with running totals.

![A whole-location insurance report with summary cards and a section per item](../../assets/screenshots/insurance-location.png)

A location report shows:

- **A header** labelled **Location Insurance Report**, with the location name (and icon), your group name, the generation date, and the **Total Items** count.
- **Three summary cards:**
  - **Total Items** — how many items are included.
  - **Total Purchase** — the combined purchase price of every item, in your group's currency.
  - **Est. Total Value** — the combined estimated replacement value, in your group's currency.
- **A section per item**, each with its type and area, name, warranty badge, purchase price, estimated value, serial number, purchase date, any warranty note, and the item's cover photo.

Items are grouped by **location**, which is made up of its **areas**. An item only appears in the report if it's been placed in an area belonging to the selected location — see [Locations & areas](../locations-and-areas/). If a location has no items, the report says **No items found for this location.**

## Photo size: thumbnails vs full

The **photo size** toggle in the toolbar controls how images appear in the report:

- **Thumbnails** — compact, in a tidy grid. Best for fitting more on a page.
- **Full size** — larger, stacked images. Best when the detail matters.

This setting applies to both modes. If an item has no photos, its photo block is simply left out.

:::note
In the **single-item** report, the photo block shows all of the item's attached images. In the **location** report, each item shows only its **cover photo** (the image marked as the item's cover), to keep the document a sensible length.
:::

## Print or save as PDF

The report is designed to print beautifully — the toolbar disappears and only the report sheet remains.

1. Set up the report the way you want it (mode, item/location, photo size).
2. Click **Print / Save PDF**.
3. Your browser's print dialog opens. From here you can:
   - Send it to a printer, or
   - Choose **Save as PDF** (or "Print to PDF") as the destination to keep a digital copy.

A few tips for a clean result:

- Inventario tries to keep each item's section from being split across a page break, so items stay readable.
- Use **Thumbnails** if you want the report to take fewer pages; use **Full size** when photo detail is important.

## Good to know

- **Currencies.** Totals and estimated values are shown in your **group currency**. An item's purchase price is shown in the currency you recorded it in. You set your group currency when you create a group, and change it later via **Group settings → Info** — see [Groups & sharing](../groups-and-sharing/).
- **Estimated value** is the item's current replacement estimate, separate from what you originally paid. Keeping it up to date makes the report more useful at claim time.
- **Sold, lost, or disposed items** are left out of reports, which only cover items you currently own.
- **Share a report.** Saving as a PDF (above) is the simplest way to hand a copy to an insurer. To export your underlying data instead, see [Backup & restore](../backup-and-restore/).
- **More report types** can appear on the Reports page over time; the Insurance Report is the one available today.
