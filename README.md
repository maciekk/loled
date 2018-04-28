# loled

**L**ist **O**f **L**ists **ED**itor

TODO: still looking for a better name, to better capture the spirit and unique
qualities of the tool.

Draws inspiration from:
* LISP - the idea that "everything is a list"
* CLEAR - a TODO list app for iOS
* Vim - tactical focus and efficiency of keyboard use; also soon will borrow
  keyboard "modes".

## Designs (in progress)

### Tagged items

Random thoughts:
- it only makes sense for items in CURRENT list to be tagged?
- once you leave current list (go up or down), reset tagged list?
- "tagged" is less a property of the items themselves, and more a property of
  "current view", closer to "current list" and "current item" signals
- display: in current non-ncurses approach, display using "- [TAG]"
        - specifically cannot simply replace the "-" since collision with
          current item indicator, which would hide the tag

### Screen layout, once use ncurses library

Thought dump:
- primary element of the UI is the display of the "current list"
- if using keyboard modes, display mode in standard location
- probably need an echo area for ephemeral output ("Saved to x...")
- likewise, need an area to capture multi-stroke/full editor input (e.g.,
  Append or Replace item)
- status might be useful too of some sort?
- long story short, might structure things like in Vim:
        - top 90% is the primary content (i.e., current list)
        - then status bar
        - then echo / long input line
- for some operations will need the content area to be split vertically, so
  that current list is on the left, while on right we have a "working pane".
  Some potential uses:
        - "quick preview of child"
        - re-assembly / in-progress re-ordering of current list
        - search results
