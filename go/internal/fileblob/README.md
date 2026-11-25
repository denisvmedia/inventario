# Fileblob

Fileblob is a copy of `gocloud.dev/blob/fileblob` with a way to use memory
fs (through Afero) instead of the OS fs.

It follows upstream's behavior as of gocloud.dev v0.49.0: `Delete` takes
`DeleteOptions` and calls `BeforeDelete` once, keys and list prefixes may not
escape the bucket root, `List` returns the MD5 of the blobs it wrote, paging
with a delimiter keeps key order, and a failing `BeforeWrite` removes its
temporary file. Some upstream features are not in this copy: the `no_tmp_dir`
and `dir_file_mode` URL options and the `IfNotExist` write precondition.
Inventario uses none of them.

When you upgrade gocloud.dev, diff upstream's `blob/fileblob` between the old
and the new version and port what applies here.
