# dstow list

<!-- dstow:short -->
What is configured: repos, packages, targets (never reads disk)
<!-- /dstow:short -->

<!-- dstow:long -->
What is configured — repos, packages, targets, exclusions, sources.
Reads only configuration: instant, side-effect free, never inspects disk.
Deployment truth lives in 'dstow status'.

Bare list shows repos (the global scope's content); naming a repo lists its
packages; naming a package lists its paths, relative to the package
directory.
<!-- /dstow:long -->
