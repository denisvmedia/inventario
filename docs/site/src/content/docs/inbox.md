---
title: Inbox
description: Find the drafts, items and batches of files you left unfinished, and finish or clear each one.
---

Items often get added in a hurry: a draft saved on the phone, an item marked
as a draft to check later, a pile of receipts added from files. The **Inbox**
collects this unfinished work in one list. Each row says why it is there and
offers the step that clears it.

Open **Inbox** from the second row of the **Inventory** section in the
sidebar. The number next to it is how many records in the group need
attention. Other places lead to the inbox too:

- The dashboard shows a **Needs attention** card with the same number and what
  it is made of, for example "2 drafts to finish · 1 item to review · 1 batch
  of files to finish". Click it to open the inbox.
- The **All Items** page shows a line with the drafts and the items to review
  and an **Open inbox** button. It leaves out the batches of files, which the
  page lists in its own **Added from files** card.

The card, the line and the number are hidden while there is nothing for them
to show.

## What is in the inbox

| Reason | What it is | What clears it |
| --- | --- | --- |
| **Draft** | A [draft of a new item](../items/#drafts-of-a-new-item), saved but not added as an item yet. | **Continue** and add the item, or **Discard**. |
| **Marked as draft** | An item whose **Save as draft** switch is on (see [Drafts of an existing item](../items/#drafts-of-an-existing-item)). | **Mark as reviewed**. |
| **Added from files** | A batch of files you [added from files](../items/#add-items-from-files) that is not finished: a file waits for its scan or could not be read, or a product waits for your decision or for its draft. | **Review** the batch and create the drafts, retry the files that failed, or remove the batch. |

The tabs above the list show all rows, only **Drafts**, only
**Items to review**, or only the batches **From files**, each with its count.
The number in the sidebar counts every record once, even one that is listed
for more than one reason.

The newest drafts come first, then the items, then the batches of files,
newest first. A long inbox is split into pages of 50 rows.

## Finish a draft

- **Continue** opens the draft in the **Add item** dialog on top of the inbox,
  with its values and files. Add the item to finish it; the draft leaves the
  inbox. Close the dialog to come back to the inbox with the draft unchanged.
- **Discard** deletes the draft after you confirm. Its files that nothing else
  uses go to the [Trash](../trash/).

See [Save as draft](../items/#save-as-draft) for how drafts are saved and
shared between devices.

## Review an item

An item marked as draft is hidden from **All Items** until **Show inactive**
is on, so the inbox is where you find it. Click its name to open it on top of
the inbox and check its details.

**Mark as reviewed** turns the item's **Save as draft** switch off, and the
item leaves the inbox. The message at the bottom of the screen has an **Undo**
button that marks it as draft again.

Marking an item as reviewed needs the item to be complete: if something is
missing, for example a price in another currency without an amount in the
group currency, the item stays in the inbox and the message says what to fix,
with an **Open item** button. An unknown purchase date or price is fine: a
reviewed item can keep them unknown, and it does not come back to the inbox
for them.

## Finish a batch of files

A batch row shows how many files the batch has, its state (**Scanning**,
**To review** or **Done**), when it was added and how far it got, for example
"1 of 3 scanned" or "2 products to review". While its files are being scanned,
the row updates by itself.

- **Review** opens the batch's review page (see
  [Review a batch](../items/#review-a-batch)), where you keep the products you
  want and create their drafts. The button reads **Open** while files are still
  being scanned, for a batch whose only work left is a file that could not be
  read, and for members who can only view.
- The trash button removes the batch after you confirm. Drafts made from it
  stay; its files that nothing else uses go to the [Trash](../trash/). A batch
  cannot be removed while its files are being scanned.

The batch leaves the inbox once every file is scanned and every product has a
draft or is left out. A file that could not be read keeps the batch here until
you retry it with **Retry unfinished** on the review page or remove the batch,
so a failed scan is not forgotten.

## Missing details

An item without a location or without tags is not unfinished, so it is not in
the inbox and not in its count. The **Missing details** section under the list
opens **All Items** with the matching filter:

- **Items without a location** turns on the **No location** filter.
- **Items without tags** shows the untagged items.

You can [save either list as a view](../items/#saved-views) to keep it one
click away.

## Who can do what

Everyone in the group sees the same inbox. Members with the **Viewer** role
see the rows but cannot continue, discard, or review them, or remove a batch;
they can open a batch to see it.
