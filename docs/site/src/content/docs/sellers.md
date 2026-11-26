---
title: Sellers
description: Record the shops and people your items were bought from, and list everything bought from one of them.
---

A **seller** is a shop or a person your group buys from: "Hardware Store",
an online shop, the neighbor who sold you a bike. Each item can name the seller
it was bought from. The seller then keeps its website and contact in one place,
and lists every item bought there, which helps when a warranty claim or a
return needs the receipt.

Sellers belong to a group, like [tags](../tags/). Every member of the group
sees the same sellers, and members who may edit items may add, edit, and delete
sellers (see [Groups & sharing](../groups-and-sharing/)).

## Open the Sellers page

Click **Sellers** in the **Group** section of the sidebar, or search for
"Sellers" in the search window at the top of every page.

Each row shows the seller's name, the host of its website (a link that opens
the site), the number of items bought from it, and its contact and note when
they are set. The count includes drafts and items that are no longer in use
(sold, lost, and so on), and leaves out items in the [Trash](../trash/).

Type in the search box to keep the sellers whose name contains the text. Upper
and lower case and extra spaces don't matter.

## Add a seller

1. Click **New seller**.
2. Fill in the fields:

   | Field | Notes |
   | --- | --- |
   | **Name** * | Up to 100 characters. Each name is used once in the group: "hardware store" and "Hardware  Store" count as the same name. |
   | **Website** | A full web address that starts with `http://` or `https://`. |
   | **Contact** | A phone number, email, or address. Up to 500 characters. |
   | **Note** | Anything worth keeping, such as a customer number or opening hours. Up to 2000 characters. |

3. Click **Create seller**.

You can also add a seller while you fill in an item, without leaving the form
(see [Name the seller of an item](#name-the-seller-of-an-item)).

## Name the seller of an item

The **Seller** picker is on the **Purchase** step when you add an item, and in
the **Purchase** section when you edit one.

1. Open the picker and choose a seller of your group.
2. If the seller is not there yet, choose **New seller** at the end of the
   list. The same dialog as on the Sellers page opens over the form. When you
   create the seller, it is selected, and what you typed in the item form stays
   as it was.
3. To remove the seller from the item, click **Clear seller** above the picker.

If the name you type for a new seller is already taken, for example because
another member added that seller a moment ago, the dialog says so and offers
**Use {name}** to pick the existing seller instead.

If another member deletes the seller while you have it picked, the picker says
the seller was deleted. Pick another one or clear the seller before you save.

The item page shows the seller in its **Details**, next to the purchase date.
The seller's name links to the list of items bought from it.

### Sellers read by AI

When [Fill with AI](../files-and-photos/#scan-an-item-from-a-photo-or-pdf-with-ai) reads a receipt
or an invoice, it reads the seller's name too, and **Seller** is one of the rows
on **Review extracted details**:

- If your group has a seller with that name, the item names it.
- If not, Inventario creates nothing on its own. The **Purchase** step shows the
  name the scan read, with a **Create seller** button that opens the dialog
  with the name filled in. If a seller with that name exists by then,
  the button offers to use it instead.

[Add items from files](../items/#add-items-from-files) works the same way: a
draft made from a receipt names your group's seller with that name, or shows
the name on the draft's **Purchase** step with the **Create seller** button.

When you draft an item from the public landing page, before you have an
account and a group, there are no sellers yet: the scan adds a "Seller: …" line
to the item's **Comments** instead.

## See everything bought from a seller

Click a seller's name on the Sellers page, or on an item's page. The
**All Items** page opens with the seller filter set and inactive items
shown, so the list holds the same items as the count.

On **All Items** the **Seller** menu in the toolbar filters by one or more
sellers; an item bought from any of them is listed. The menu appears once your
group has a seller. See [Items](../items/#the-all-items-page-search-filter-sort).

## Edit or delete a seller

Point at a row on the Sellers page and use the pencil to edit the seller or the
bin to delete it.

Deleting a seller cannot be undone. The confirmation says how many items name
the seller. The items themselves stay, and no longer name a seller; this
includes the items in the Trash. A link to the deleted seller's items opens an
empty list, with **Deleted seller** in the **Seller** menu so you can clear the
filter.

## Sellers in reports and backups

The [insurance report](../reports/) shows each item's seller under its purchase
date.

A backup carries the group's sellers, and a restore links each item to the
seller with the same name, creating the sellers the group does not have (see
[Backup & restore](../backup-and-restore/#sellers)).
