# dstow theme slots

<!-- dstow:short -->
Describe every color slot: what it colors and its consumers, plus the value grammar
<!-- /dstow:short -->

<!-- dstow:long -->
Every generic slot and what it colors, each name shown in its own effective
style. dstow's internals — package states, check classes, severity prefixes,
prose roles — reach these slots through a fixed code-owned mapping (§7.2); each
description names the slot's consumers.

Slot values use git's color.* grammar: whitespace-separated words, in any
order. The first color word is the foreground, the second the background, a
third is an error. Color words are the 8 basics (black red green yellow blue
magenta cyan white), their bright* variants, an integer 0-255 (256-color), or
#RRGGBB hex. Write 'normal red' to set a background without touching the
foreground. Attributes, any number: bold dim italic ul blink reverse strike,
each negatable with no or no- (a negation renders as nothing — a themed slot
replaces its default wholesale); 'reset' comes first.

normal leaves a channel to the TERMINAL, not to dstow's default: a slot set to
normal replaces its default wholesale (§7.3 top-wins) and renders plain — the
only way to keep dstow's default for a slot is to leave it undeclared. default
differs: it emits the terminal-default code (SGR 39/49) rather than nothing.
<!-- /dstow:long -->
