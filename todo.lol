DONE 38
TRASH 44
node 1
root
2 9 12 14 16 17 19 20 21 22 23 24 26 28 30 31 33 34 37 38 44 
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
get rid of 'ds' and 'vd' singletons!

node 20
add high-level overview of classes, roles, design

node 21
show filename instead of "root"

node 22
sug: "recently visited files" could be item under root

node 23
sug: vertical scrolling for lists taller than window

node 24
sug: allow multiple user-set Targets
25 
node 26
bug: cursor not adjusted on fold creation
27 
node 28
sug: reliable way to set 'dirty'
29 
node 30
sug: have "Pull from Target" command (reverse of Move to Target)

node 31
sug: track navigation history, allow "go back"
32 
node 33
sug: undo functionality

node 34
bug: off-by-one in gocui color specification?
35 36 
node 37
sug: RPG-like reward system for completing work

node 38
[[DONE]]
39 40 43 
node 44
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

node 25
pattern on Vim's marks (i.e., letter identifiers)

node 27
repro: only if items are at start of list

node 29
often forget to set it when adding new datastore-modifying code

node 32
esp important since now have random-access hops (e.g., marks)

node 35
e.g., compare against values specified by standard

node 36
is it maybe issue w/underlying termbox-go?

node 39
bug: crash on replace on list with no items?

node 40
confusing handling of Enter in dialog
41 42 
node 43
sug: add useful Vim folding within files

node 4
modified status

node 5
subtree summary: item count, tree depth

node 6
overall datastore stats

node 7
build date of loled (i.e., version)

node 41
repro: enter text, place cursor in MIDDLE of line, hit Enter to "finish"

node 42
maybe always show 2 lines, to make obvious

