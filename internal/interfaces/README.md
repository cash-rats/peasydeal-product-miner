# internal/interfaces

This folder is for cross-domain interfaces to avoid cyclic imports between `internal/app/<domain>` packages.

Guideline: define interfaces here, depend on interfaces in other domains, and wire concrete implementations via FX in the caller’s module.
