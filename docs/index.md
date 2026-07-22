# dstow manual

The complete dstow documentation, reachable from the command line alone.

Every topic prints itself and lists the topics beneath it, so the tree is
navigable without `--help`: run `dstow manual <topic>`, then follow what it
lists.

## Topics

- `commands` — one page per dstow command, mirroring the command tree. Each
  page is also where that command's help text comes from, so
  `dstow manual commands repo add` and `dstow repo add --help` are the same
  content.

The rest of the tree is still being written. Until it lands, per-command help
is the fullest surface:

    dstow --help
    dstow <command> --help
