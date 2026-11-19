; Lastab's additions to MyGo's Windows installer (mygo.json windows.installerInclude).
;
; Settings > Agent access > "Install command-line tool" adds the app's bin
; folder, $INSTDIR\bin, to the user's PATH (HKCU\Environment). Uninstalling
; takes it out again, so no PATH entry points at a folder that is gone.
; Updates replace the files without the uninstaller and keep the entry.

!include "LogicLib.nsh"

!macro customUnInstall
  Push "$INSTDIR\bin"
  Call un.LastabRemoveFromPath
!macroend

; Removes the folder on the stack from the user's PATH, matching entries as
; typed or with their variables expanded, ignoring case and a trailing
; backslash, and tells running programs. Leaves PATH alone when it is too
; long for NSIS's strings, which would cut it short.
Function un.LastabRemoveFromPath
  Exch $0 ; the folder
  Push $1 ; PATH, then what is left of it
  Push $2 ; the entries kept
  Push $3 ; an entry
  Push $4 ; where the entry ends
  Push $5 ; a character, then the entry without a trailing backslash
  Push $6 ; the entry expanded
  Push $7 ; a length
  Push $8 ; 1 once an entry is removed
  Push $9

  ClearErrors
  ReadRegStr $1 HKCU "Environment" "Path"
  ${If} ${Errors}
  ${OrIf} $1 == ""
    Goto done
  ${EndIf}
  StrLen $7 $1
  IntOp $9 ${NSIS_MAX_STRLEN} - 1
  ${If} $7 >= $9
    DetailPrint "PATH is too long to edit; leaving $0 in it"
    Goto done
  ${EndIf}

  Push $0
  Call un.LastabTrimSlash
  Pop $0
  StrCpy $2 ""
  StrCpy $8 0
  ${Do}
    ; $3 is the entry up to the next ";", $1 what follows it.
    StrCpy $4 0
    StrLen $7 $1
    ${DoWhile} $4 < $7
      StrCpy $5 $1 1 $4
      ${IfThen} $5 == ";" ${|} ${ExitDo} ${|}
      IntOp $4 $4 + 1
    ${Loop}
    StrCpy $3 $1 $4
    IntOp $9 $4 + 1
    StrCpy $1 $1 "" $9

    Push $3
    Call un.LastabTrimSlash
    Pop $5
    ExpandEnvStrings $6 $5
    ${If} $5 == $0 ; StrCmp: case doesn't matter
    ${OrIf} $6 == $0
      StrCpy $8 1
    ${ElseIf} $3 != ""
      ${If} $2 == ""
        StrCpy $2 $3
      ${Else}
        StrCpy $2 "$2;$3"
      ${EndIf}
    ${EndIf}
  ${LoopWhile} $4 < $7

  ${If} $8 == 1
    ${If} $2 == ""
      DeleteRegValue HKCU "Environment" "Path"
    ${Else}
      ; The user's PATH is REG_EXPAND_SZ, as Windows and Lastab write it.
      WriteRegExpandStr HKCU "Environment" "Path" $2
    ${EndIf}
    ; WM_SETTINGCHANGE to every top-level window (HWND_BROADCAST).
    SendMessage 0xFFFF 0x1A 0 "STR:Environment" /TIMEOUT=5000
  ${EndIf}

  done:
  Pop $9
  Pop $8
  Pop $7
  Pop $6
  Pop $5
  Pop $4
  Pop $3
  Pop $2
  Pop $1
  Pop $0
FunctionEnd

; Takes the backslashes and slashes off the end of the string on the stack.
Function un.LastabTrimSlash
  Exch $0
  Push $1
  ${Do}
    StrCpy $1 $0 1 -1
    ${If} $1 == "\"
    ${OrIf} $1 == "/"
      StrCpy $0 $0 -1
    ${Else}
      ${ExitDo}
    ${EndIf}
  ${Loop}
  Pop $1
  Exch $0
FunctionEnd
