Refactored Config.UnmarshalJSON in pkg/devcontainer/config.go to use the embedded *Alias pattern (matching BuildConfig.UnmarshalJSON in build.go). This eliminates:
- 37-line duplicate Alias struct with all Config fields re-declared
- 35-line manual field-copy block (c.X = aux.X for every field)

The entrypoint string-or-array handling is preserved using a json.RawMessage field in the aux struct. New fields added to Config will automatically be unmarshaled — no triple-site maintenance.

shellspec sandbox_smoke failures are pre-existing: caused by read-only go cache filesystem in the sandbox, not by this change.
