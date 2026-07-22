# dstow snippet rc

<!-- dstow:short -->
The shell-rc bootstrap: installs dstow iff absent — silent and network-free whenever dstow is already present
<!-- /dstow:short -->

<!-- dstow:long -->
Print the shell-rc bootstrap to stdout: it installs dstow only when dstow is
absent, and is silent and network-free whenever dstow is already present.

Appending it to an rc file is yours to do — dstow never edits rc files.
<!-- /dstow:long -->

<!-- dstow:examples -->
  dstow snippet rc >> ~/.bashrc
<!-- /dstow:examples -->
