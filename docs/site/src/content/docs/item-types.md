---
title: Item types
description: Keep your group's own list of item types, with names and icons, and move items from one type to another.
---

Every item has a **type**, such as "Electronics" or "Furniture". The type
gives the item its icon when it has no photo, and the **Type** filter on
**All Items** lists items by it.

Item types belong to a group, like [tags](../tags/) and
[sellers](../sellers/). Every member of the group sees the same types, and
members who may edit items may add, rename, and delete types (see
[Groups & sharing](../groups-and-sharing/)). A type stays with the group when
the member who added it leaves or deletes their account.

A new group starts with six **built-in** types: **Appliance**,
**Electronics**, **Equipment**, **Furniture**, **Clothes**, and **Other**.
Each member sees their names in their own language. A type you add keeps the
name you gave it, in every language.

A group admin can add the [Collection set](../groups-and-sharing/#add-the-collection-set)
of types for collections: **Art**, **Books**, **Coins**, **Stamps**,
**Records**, **Watches**, and **Cards**, created with English names. They are
ordinary types you can rename, give another icon, or delete.

## Open the Item types page

Click **Item types** in the **Group** section of the sidebar, or search for
"Item types" in the search window at the top of every page.

The built-in types come first, then your own types by name. Each row shows the
type's icon and name, a **Built-in** badge on the six built-in types, and the
number of items that have the type. The count includes drafts and items that
are no longer in use (sold, lost, and so on). Items in the [Trash](../trash/)
are counted on their own, as "N in Trash".

Type in the search box to keep the types whose name contains the text. Upper
and lower case don't matter.

## Add a type

1. Click **New type**.
2. Enter a **Name**, up to 50 characters. Each name is used once in the group:
   "vehicle" and "Vehicle" count as the same name. A built-in type that keeps
   its default name holds its English name, so "Electronics" is taken while
   the built-in Electronics type is not renamed.
3. Pick an **Icon**.
4. Click **Create type**.

A group keeps up to 100 item types. When it has that many, delete or merge
types before you add another.

You can also add a type while you fill in an item: open the **Type** picker
and choose **New type** at the end of the list. The same dialog opens over the
form, and the type you create is selected. If the name is already taken, the
dialog says so and offers **Use {name}** to pick the existing type instead.

## Rename a type or change its icon

Point at a row and click the pencil. Items keep their type when you rename it,
so they show the new name at once.

A built-in type has an empty name field until you rename it. Leave it empty to
keep the default name, which each member sees in their own language. A name you
enter is shown as typed, in every language.

## Delete or merge types

Point at a row and click the bin.

- When no item and no [draft](../items/#save-as-draft) has the type, confirm
  and the type is removed.
- When items have it, the dialog says how many, Trash included, and asks for
  the type they move to. Pick one and click **Move items and delete**: every
  one of those items gets the type you picked, then the type is deleted.
  Drafts that have the type are counted too and move with the items.
- When only drafts have it, the dialog says how many and the button reads
  **Move drafts and delete**.

A draft that is open in another window while its type moves cannot save over
the change: that window says the draft changed elsewhere.

To merge two types, delete one and move its items to the other.

A group always keeps at least one type, so its last type cannot be deleted. The
built-in types can be deleted like any other.

If another member deletes a type while you have it picked in an item form, the
**Type** picker says the type was deleted. Pick another one before you save.

## See every item of a type

Click a type's name on the Item types page, or on an item's page under the
item's name. The **All Items** page opens with the type filter set and inactive
items shown, so the list holds the same items as the count. On **All Items**
the **Type** menu in the toolbar lists your group's types (see
[Items](../items/#the-all-items-page-search-filter-sort)).

## Custom fields of a type

A [custom field](../custom-fields/#offer-a-field-on-item-types) can be offered
on the items of some types, with a default value for each. Picking such a type
in an item form shows those fields and fills in their defaults. Deleting a type
removes it from the fields that were offered on it.

## Item types read by AI

When [Fill with AI](../files-and-photos/#scan-an-item-from-a-photo-or-pdf-with-ai)
reads a photo or a document, it picks the item's type from your group's types,
the ones you added included.

When you draft an item from the public landing page, before you have an
account and a group, the **Type** picker offers the six built-in types.

## Item types in reports and backups

The [insurance report](../reports/) shows each item's type by the name your
group gave it.

A backup carries the group's item types, and a restore gives each item the
matching type of the group it restores into, creating the types the group
does not have (see [Backup & restore](../backup-and-restore/#item-types)).
