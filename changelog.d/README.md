# Changelog fragments

A pull request adds its changelog entries here, as one new file, and leaves
`CHANGELOG.md` alone. Two pull requests that add files never conflict. The
release pull request moves every fragment into `CHANGELOG.md` and deletes it.
[CONTRIBUTING.md](../CONTRIBUTING.md#changelog) says which changes need an
entry and how to write one.

## File name

Name the file after your branch, with `/` replaced by `-`: branch
`feat/1607-collection-set` adds `feat-1607-collection-set.md`. A name uses
lowercase letters, digits, `.`, `_` and `-`, and ends in `.md`.

## Content

The entries exactly as they will read in `CHANGELOG.md`, link included, under
one or more section headings:

```markdown
### Changed

- An item that leaves out `converted_original_price` is no longer rejected with
  "cannot be blank". The value counts as 0, and the price rule still decides
  which prices an item needs
  ([#2781](https://github.com/denisvmedia/inventario/pull/2781)).

### Security

- Prices are read with `shopspring/decimal` 1.6.0. A price with a huge exponent,
  like `1e1000000000`, is now rejected. Before, such a value in a request could
  hang the server or use gigabytes of memory
  ([#2781](https://github.com/denisvmedia/inventario/pull/2781)).
```

- A heading is one of `### Added`, `### Changed`, `### Deprecated`,
  `### Removed`, `### Fixed`, `### Security` and `### Operators`, spelled
  exactly so.
- Each heading has at least one entry. An entry is a list item that starts
  with `-` and a space, and its continuation lines are indented by two
  spaces.
- Nothing comes before the first heading, and there are no other headings.

To change an entry that is already here, edit its file, whichever branch
added it.

`./scripts/changelog-assemble.sh --check` validates every fragment; Markdown
Lint runs it on each pull request that changes a Markdown file. markdownlint
skips the fragments, as it skips `CHANGELOG.md`.
