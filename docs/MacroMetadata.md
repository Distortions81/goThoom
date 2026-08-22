# Legacy macro metadata

Legacy `.mac` files can include an optional metadata block at the top of the
file. Metadata is presentation-only: it does not change how the macro runs.

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

`// Metadata` is an optional marker. Put one field per `//` comment line. Field
names are case-insensitive, leading and trailing value whitespace is ignored,
and a later non-empty copy of a field replaces an earlier value. Fields may be
omitted. If `Name` is omitted, the client uses the macro filename.

| Field | Purpose |
| --- | --- |
| `Name` | Friendly macro name shown in the library. |
| `Version` | Macro release or revision identifier. |
| `Tags` | Comma-separated categories to help identify the macro. |
| `Desc` | Short description; appears alongside the macro name when space permits. |
| `Author` | Macro author or maintainer. |
| `License` | License and year for the source. |
| `Website` | Originating project or author site. |
| `Update` | Direct URL for the macro's update source, when available. |

Click the **i** button in **Actions → Legacy Macros** to see the full metadata,
commands, and hotkeys. The Global and Player enablement settings are stored
separately and are not part of a macro's metadata.
