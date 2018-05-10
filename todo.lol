DONE 43
TRASH 49
node 1
root
2 9 12 14 16 17 19 20 21 22 23 24 25 26 27 29 31 33 34 36 37 38 39 42 43 49 
node 2
add Info/Status pane (third one)
3 8 
node 9
add 'modified' indication
10 11 
node 12
better / cleaner way to print color strings
13 
node 14
sug: more ASCII art
15 
node 16
sug: option to auto-save after every command? (safety)

node 17
sug: if tagged items, delete should apply to them all
18 
node 19
add high-level overview of classes, roles, design

node 20
get rid of 'ds' and 'vd' singletons!

node 21
show filename instead of "root"

node 22
sug: "recently visited files" could be item under root

node 23
sug: have '/' like search, so can traverse faster in list

node 24
sug: have Emacs-like 1 trough 0 (for 10), to quickly jump to item #

node 25
sug: bindings to go to top, middle, bottom of screen lines

node 26
sug: vertical scrolling for lists taller than window

node 27
sug: allow multiple user-set Targets
28 
node 29
bug: cursor not adjusted on fold creation
30 
node 31
sug: reliable way to set 'dirty'
32 
node 33
sug: have "Pull from Target" command (reverse of Move to Target)

node 34
sug: track navigation history, allow "go back"
35 
node 36
sug: command to go back to root

node 37
sug: have ability to cancel out of Replace ('Esc'?)

node 38
sug: undo functionality

node 39
bug: off-by-one in gocui color specification?
40 41 
node 42
sug: RPG-like reward system for completing work

node 43
[[DONE]]
44 45 48 
node 49
[[TRASH]]

node 3
what to show
4 5 6 7 
node 8
position: on top of log window

node 10
use '*' (in pane title), like in Vim

node 11
longer-term: show in (new) info pane

node 13
specifically, avoid use of ANSI escape codes

node 15
e.g., ASCIIfy "so excited" happy face

node 18
vs just the current item always

node 28
pattern on Vim's marks (i.e., letter identifiers)

node 30
repro: only if items are at start of list

node 32
often forget to set it when adding new datastore-modifying code

node 35
esp important since now have random-access hops (e.g., marks)

node 40
e.g., compare against values specified by standard

node 41
is it maybe issue w/underlying termbox-go?

node 44
bug: crash on replace on list with no items?

node 45
confusing handling of Enter in dialog
46 47 
node 48
sug: add useful Vim folding within files

node 4
modified status

node 5
subtree summary: item count, tree depth

node 6
overall datastore stats

node 7
build date of loled (i.e., version)

node 46
repro: enter text, place cursor in MIDDLE of line, hit Enter to "finish"

node 47
maybe always show 2 lines, to make obvious

