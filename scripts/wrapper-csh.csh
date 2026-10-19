# Update CLI shell wrapper (csh/tcsh); source through the installed alias.
set _ucl_cmd = ""
if ( $#argv > 0 ) set _ucl_cmd = "$argv[1]"
if ( "$_ucl_cmd" != "" && "$_ucl_cmd" != "update" ) then
  update-cli $argv:q
else
  set _ucl_root = "$cwd"
  while ( "$_ucl_root" != "/" && ! -f "$_ucl_root/.update-cli/config.json" )
    set _ucl_root = "`dirname "$_ucl_root"`"
  end
  if ( ! -f "$_ucl_root/.update-cli/config.json" ) then
    update-cli $argv:q
  else
    set _ucl_inside = 0
    if ( "$cwd" == "$_ucl_root/current" ) set _ucl_inside = 1
    if ( "$cwd" =~ "$_ucl_root/current/*" ) set _ucl_inside = 1
    if ( $_ucl_inside == 1 ) then
      set _ucl_before = "$cwd"
      cd "$_ucl_root"
      update-cli $argv:q
      set _ucl_status = $status
      if ( $_ucl_status == 0 ) then
        cd "$_ucl_root/current"
      else
        cd "$_ucl_before"
      endif
      unset _ucl_before _ucl_status _ucl_inside
    else
      update-cli $argv:q
      unset _ucl_inside
    endif
  endif
  unset _ucl_root
endif
unset _ucl_cmd
