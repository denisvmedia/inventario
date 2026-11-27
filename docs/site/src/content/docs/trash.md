---
title: Trash
description: Restore deleted items and files, or delete them for good.
---

When you delete an item or a file, Inventario moves it to the **Trash**
instead of removing it at once. It waits there for 30 days, so a wrong click
or a change of mind costs nothing. After that it is deleted for good. The
person who runs your Inventario server can change the number of days; the
Trash page always shows the current value.

Open **Trash** from the last row of the **Group** section in the sidebar.

## What goes to the trash

- An item you delete, from its page or with a bulk delete on **All Items**.
  The files attached only to that item go with it. A file that is also
  attached to another item, a location, or an area stays where it is.
- A file you delete from the **Files** page or from its details panel.
- The items inside a location or area that you delete with
  **Delete everything** (see [Locations & areas](../locations-and-areas/#delete-a-location-or-area)).
  The location or area itself is deleted at once and does not go to the trash,
  and neither do the files attached directly to it.

Backups are not part of this: deleting an export removes it for good (see
[Backup & restore](../backup-and-restore/#manage-and-delete-exports)).

Right after you delete items, the message at the bottom of the screen has an
**Undo** button that restores them straight away. If a restore gets no answer
for some of them, a warning says how many may have been restored; check the
trash for the rest.

## While something is in the trash

An item in the trash is out of the way:

- It does not appear in item lists, search, the dashboard, reports, the
  Upcoming page, or the warranty, lent-out, in-service, and maintenance pages,
  and it is left out of totals and counts.
- You get no warranty, loan, maintenance, or item reminders for it, and it is
  left out of the weekly digest.

Nothing about it is lost. Its history, loans, service records, maintenance
schedules, tags, and attachments are kept and come back when you restore it.

A file in the trash still counts toward your group's storage until it is
deleted for good. **Group settings → Data & storage** shows how much of the
used space is in the trash, with a link to the Trash page.

## The Trash page

The page has two tabs:

- **Items** lists deleted items, with how many files went to the trash with
  each one and how much space they take.
- **Files** lists files you deleted on their own, with the items, locations,
  areas or drafts they were attached to.

Each row shows when it was deleted and by whom, and the date it will be
deleted for good.

What you can do depends on your [role](../groups-and-sharing/#members-and-roles)
in the group:

| Role | On the Trash page |
| --- | --- |
| **Viewer** | Sees the lists. |
| **User** | Also restores items and files. |
| **Administrator**, **Owner** | Also deletes entries for good and empties the trash. |

Each restore and each delete for good is recorded in the audit log of the
server, with who did it and which items and files it restored or deleted. So
is the automatic deletion of entries whose time in the trash is up. The people
who run the server can read that log; group members don't see it.

## Restore

Click **Restore** on a row, or tick several rows and click **Restore** in the
bar that appears.

A restored item comes back as the same item: the same page address, the same
[asset ID](../items/#asset-ids-and-qr-labels) (so printed QR labels still
work), its history, and the files that went to the trash with it.

- A file you had deleted on its own before you deleted the item stays in the
  trash. Restore it from the **Files** tab.
- A file that went to the trash with an item is not listed on the **Files**
  tab. It comes back when you restore the item.
- If the item's area was deleted while the item was in the trash, the item
  comes back without a location, and Inventario tells you so. Open the item
  and file it in another area (see
  [Locations & areas](../locations-and-areas/)). A deleted location or area is
  never recreated.

## Delete for good

Administrators and owners can delete entries before their time is up:

- **Delete now** on a row deletes that item or file.
- Tick several rows and click **Delete now** in the bar to delete them
  together.
- **Empty trash** at the top of the page deletes everything in the trash.

Each of these asks you to confirm first.

Deleting an item for good also deletes its history and the files that went to
the trash with it. A file that another item, location, or area still uses is
kept there. A file that is not in the trash, such as one you restored on its
own while the item was there, is never deleted with the item.

:::caution
Deleting for good cannot be undone. If you might need something later, leave
it in the trash or make a [backup](../backup-and-restore/) first.
:::

## Trash and backups

Backups leave the trash out: an export contains only items and files that are
not in the trash. When you restore a backup:

- **Full replace** empties the trash first.
- A merge restore leaves an item or file that is in your trash where it is and
  skips it. Restore it from the Trash page instead.

See [Backup & restore](../backup-and-restore/) for the restore options.
