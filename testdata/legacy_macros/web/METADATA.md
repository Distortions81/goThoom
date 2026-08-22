# Legacy macro metadata

Put this optional comment block at the top of a `.mac` file. It affects the
macro library presentation only; it does not change macro behavior.

```text
// Metadata
// Name: <macro name>
// Version: <version>
// Tags: <healer/fighter/sharing/sunstone/anything>
// Desc: <macro description>
// Author: <author name>
// License: <license name> (year)
// Website: <website or author site>
// Update: <macro update URL>
```

Every field is optional. `Name` falls back to the filename. `Desc` appears
beside the name when there is room, and the **i** button shows all metadata,
commands, and hotkeys. Use `Website` for the originating or author site and
`Update` for the direct macro source URL when available.
