# --tui starts the terminal client in place of the window and hands it every
# other argument, in order and whole. It does that before anything needs a
# display, and this test gives it none. The terminal client here is a stand-in
# that says what it was given and exits with a code of its own, which has to
# come back as the program's.
file(MAKE_DIRECTORY "${WORK}")
set(stand_in "${WORK}/whatsapptui")
file(WRITE "${stand_in}" "#!/bin/sh\nprintf '%s\\n' \"$@\"\nexit 7\n")
file(CHMOD "${stand_in}" PERMISSIONS OWNER_READ OWNER_WRITE OWNER_EXECUTE)

execute_process(
    COMMAND "${CMAKE_COMMAND}" -E env
        --unset=DISPLAY --unset=WAYLAND_DISPLAY --unset=QT_QPA_PLATFORM
        "WHATSAPPGO_TUI=${stand_in}"
        "${APP}" --profile test-marker --tui --socket "/no such/socket"
    RESULT_VARIABLE result
    OUTPUT_VARIABLE output
    ERROR_VARIABLE errors
)

if(NOT result EQUAL 7)
    message(FATAL_ERROR "--tui did not hand over to the terminal client (exit ${result}): ${errors}")
endif()

if(NOT output STREQUAL "--profile\ntest-marker\n--socket\n/no such/socket\n")
    message(FATAL_ERROR "The terminal client was given:\n${output}")
endif()
