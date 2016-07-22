# ved
Visual Ed. A new editor, 'cuz why not?

Overview:
=========

General idea is that this is a "Visual line EDitor", so text is modified by 
entering lines of commands. Curses will be used to display, Curses::UI 
includes a simple text editor, which we will use to enter the commands. This 
be in the lower portion of the display. The upper portion will show the 
"current" state of the file, with the last command's effect highlighted.

Some commands, such as "w" will result in the truncation of the command 
history. Moving the cursor up through the history will have the effect of 
undoing any commands that are below the cursor. Moving the cursor down will 
re-do. 

The implementation will be something along the lines of use Curses::UI to
present an automatic view of the file in the top half of the display, and a
simple text editor in the bottom half. Periodically, perhaps as often as after
every keystroke, a parser written in Marpa::R2 will be run against the data in
the original file and the commands in the editor window through the line that
the cursor is on, and then displayed in the upper window. 

As an optimzation, we can keep checkpoints of the file state for ever so often
in the command history. This would let us simply implement a fast incremental
update of the display.

Color will be used in the upper portion to highlight changes, and to display
syntax as available.

Completion will be implemented with some plugin architecture, but will by
default just complete against words taken from the input and current state of
the edited file.

Editor syntax:
==============

<pre>
(.)a                    append text before addressed line
(.,.)c                  change lines (replace)
(.,.)d                  delete lines
f filename              change the filename to filename
(1,$)g/re/command-list  apply command-list to each line matching the re
(1,$)G/re/              'interactively' edit addressed lines matching re
(.)i                    insert text before addressed line
(.,+1)j                 join lines into a single line
(.)kL                   mark line with letter 'L'
(.)lL command-list      save a command list to letter 'L'
(.,.)m(.)               move lines to after destination
q                       quit
($)r file               read contents of 'file' and append after line
(.,.)s/re/text/{,g,n}   search in range for re, replace with text if found.
                        g: global, replace all instances
                        n: (number) replace n'th instance
(.,.)t(.)               transfer (copy) selected lines after addressed
(1,$)v/re/command-list  apply command-list to lines not matching re
(1,$)V/re/              'interactively' edit addressed lines not matching re
w                       write the file. This truncates the command history
wq                      write and then quit
(+1)                    change the addressed line
'L                      change addressed line to one marked 'L'
/re/                    change addressed line to one matching re
(.)< command            insert stdout from command after the addressed
(.,.)> command          pipe selected lines into command and display
(.,.)| command          pipe selected lines into command and replace
(.,.)||(.) command      pipe selected lines into command and append
!!                      repeat previous command
!xN                     repeat previous command N times
=L                      run the command list named 'L'
</pre>

Command list:
=============

Each command in command-list must be on a separate line, and every line except 
for the last must be terminated by a backslash (\). Any commands are allowed 
except for g, G, l, v and V.
