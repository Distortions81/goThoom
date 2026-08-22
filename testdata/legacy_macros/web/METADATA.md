# Add information to your macro

You can put this information at the very top of a `.mac` file. It helps people
understand what the macro is in the macro library. It does not change how the
macro works.

## Template

Copy this block to the top of your macro. Replace the words inside `< >` and
remove any lines you do not need.

```text
// Metadata
// Name: <macro name>
// Version: <version>
// Tags: <tag one>, <tag two>, <tag three>
// Desc: <short description>
// Author: <your name>
// License: <license and year>
// Website: <website URL>
// Update: <direct update URL>
```

## Example

Here is a real example from the bundled Right-Clicker macro:

```text
// Metadata
// Name: Right-Clicker
// Version: 2.0.1
// Tags: input, fighter, healer
// Desc: Calls macros from right-click and wheel actions.
// Author: Gorvin
// Website: http://gorvin.50webs.com/
// Update: http://gorvin.50webs.com/macros/RC2.txt
```

You only need the lines you want.

## What each line means

- `Name` is the name shown in the macro library. Without it, the file name is
  shown instead.
- `Tags` are short labels that describe the macro. Separate each tag with a
  comma, like `input, fighter, healer`.
- `Desc` is a short explanation of what the macro does.
- `Author` says who made the macro.
- `Version`, `License`, `Website`, and `Update` are optional extra details.

Open **Actions → Legacy Macros**, then click the **i** button to see this
information for a macro.
