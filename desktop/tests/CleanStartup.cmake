execute_process(
    COMMAND "${CMAKE_COMMAND}" -E env
        QT_QPA_PLATFORM=offscreen
        "${APP}" --smoke-test
    RESULT_VARIABLE startup_result
    ERROR_VARIABLE startup_stderr
)

if(NOT startup_result EQUAL 0)
    message(FATAL_ERROR "Desktop smoke startup failed: ${startup_stderr}")
endif()

if(startup_stderr MATCHES "qt\\.multimedia|VDPAU|[Ff]ailed to create drawable")
    message(FATAL_ERROR "Desktop startup emitted a known integration warning: ${startup_stderr}")
endif()
